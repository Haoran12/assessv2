package service

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const (
	importTemplateDirectScore  = "direct-score"
	importTemplateOrganization = "organization"
	importTemplateEmployee     = "employee"
	maxImportRows              = 5000
)

type ImportExportService struct {
	db           *gorm.DB
	auditRepo    *repository.AuditRepository
	scoreService *ScoreService
}

type DirectScoreImportPreviewInput struct {
	YearID      uint
	PeriodCode  string
	ModuleID    uint
	Overwrite   bool
	FileName    string
	FileContent []byte
}

type DirectScoreImportPreviewRow struct {
	RowNo      int      `json:"rowNo"`
	ObjectID   uint     `json:"objectId"`
	ObjectName string   `json:"objectName"`
	Score      float64  `json:"score"`
	Remark     string   `json:"remark"`
	Status     string   `json:"status"`
	Messages   []string `json:"messages"`
}

type DirectScoreImportPreviewResult struct {
	YearID      uint                          `json:"yearId"`
	PeriodCode  string                        `json:"periodCode"`
	ModuleID    uint                          `json:"moduleId"`
	Overwrite   bool                          `json:"overwrite"`
	TotalRows   int                           `json:"totalRows"`
	ValidRows   int                           `json:"validRows"`
	WarningRows int                           `json:"warningRows"`
	InvalidRows int                           `json:"invalidRows"`
	Rows        []DirectScoreImportPreviewRow `json:"rows"`
}

type DirectScoreImportConfirmRow struct {
	ObjectID uint    `json:"objectId"`
	Score    float64 `json:"score"`
	Remark   string  `json:"remark"`
}

type DirectScoreImportConfirmInput struct {
	YearID     uint                          `json:"yearId"`
	PeriodCode string                        `json:"periodCode"`
	ModuleID   uint                          `json:"moduleId"`
	Overwrite  bool                          `json:"overwrite"`
	Rows       []DirectScoreImportConfirmRow `json:"rows"`
}

type DirectScoreImportConfirmResult struct {
	Requested int `json:"requested"`
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Skipped   int `json:"skipped"`
	Imported  int `json:"imported"`
}

type ExportWorkbookInput struct {
	YearID         uint
	PeriodCode     string
	ObjectCategory string
}

type parsedDirectImportRow struct {
	RowNo      int
	ObjectID   uint
	ObjectName string
	Score      float64
	Remark     string
	Errors     []string
}

type exportSummaryRow struct {
	ObjectID       uint
	ObjectName     string
	ObjectType     string
	ObjectCategory string
	FinalScore     float64
	ExtraPoints    float64
	TriggerMode    string
	CalculatedAt   int64
}

type exportDetailRow struct {
	ObjectID      uint
	ObjectName    string
	ModuleCode    string
	ModuleKey     string
	ModuleName    string
	RawScore      float64
	WeightedScore float64
	SortOrder     int
}

type exportVoteStatRow struct {
	ModuleName  string
	GroupName   string
	GradeOption string
	VoteCount   int
}

func NewImportExportService(db *gorm.DB, auditRepo *repository.AuditRepository, scoreService *ScoreService) *ImportExportService {
	return &ImportExportService{
		db:           db,
		auditRepo:    auditRepo,
		scoreService: scoreService,
	}
}

func (s *ImportExportService) GenerateTemplate(templateType string) (string, []byte, error) {
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case importTemplateDirectScore:
		return buildDirectScoreTemplate()
	case importTemplateOrganization:
		return buildOrganizationTemplate()
	case importTemplateEmployee:
		return buildEmployeeTemplate()
	default:
		return "", nil, ErrInvalidImportTemplateType
	}
}

func (s *ImportExportService) PreviewDirectScoreImport(
	ctx context.Context,
	claims *auth.Claims,
	input DirectScoreImportPreviewInput,
) (*DirectScoreImportPreviewResult, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ModuleID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}
	if len(input.FileContent) == 0 {
		return nil, ErrImportFileRequired
	}

	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}

	module, err := loadModuleByPeriodTx(s.db.WithContext(ctx), input.ModuleID, "direct", input.YearID, periodCode)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrInvalidScoreModule
		}
		return nil, fmt.Errorf("failed to query import module: %w", err)
	}
	if module.MaxScore == nil || *module.MaxScore <= 0 {
		return nil, ErrInvalidScoreModule
	}

	parsedRows, err := parseDirectScoreImportRows(input.FileContent)
	if err != nil {
		return nil, err
	}
	if len(parsedRows) > maxImportRows {
		return nil, ErrImportRowLimitExceeded
	}

	objectIDs := make([]uint, 0, len(parsedRows))
	for _, row := range parsedRows {
		if row.ObjectID > 0 {
			objectIDs = append(objectIDs, row.ObjectID)
		}
	}
	objectMap, err := s.loadAssessmentObjectMap(ctx, input.YearID, objectIDs)
	if err != nil {
		return nil, err
	}
	existingMap, err := s.loadExistingDirectScoreObjectMap(ctx, input.YearID, periodCode, input.ModuleID, objectIDs)
	if err != nil {
		return nil, err
	}

	seenObjectIDs := make(map[uint]int, len(parsedRows))
	result := &DirectScoreImportPreviewResult{
		YearID:     input.YearID,
		PeriodCode: periodCode,
		ModuleID:   input.ModuleID,
		Overwrite:  input.Overwrite,
		Rows:       make([]DirectScoreImportPreviewRow, 0, len(parsedRows)),
	}

	for _, row := range parsedRows {
		messages := make([]string, 0, 4)
		messages = append(messages, row.Errors...)

		if row.ObjectID > 0 {
			seenObjectIDs[row.ObjectID]++
			if seenObjectIDs[row.ObjectID] > 1 {
				messages = append(messages, "文件内对象ID重复")
			}
			object, exists := objectMap[row.ObjectID]
			if !exists {
				messages = append(messages, "考核对象不存在或非当前年度激活对象")
			} else {
				row.ObjectName = object.ObjectName
				if !scope.allowsDetailObject(row.ObjectID) {
					messages = append(messages, "当前用户无该对象导入权限")
				}
			}
			if _, exists := existingMap[row.ObjectID]; exists {
				if input.Overwrite {
					messages = append(messages, "已存在记录，确认导入时将执行更新")
				} else {
					messages = append(messages, "已存在记录，确认导入时将被跳过")
				}
			}
		}

		if row.Score < 0 || row.Score > *module.MaxScore {
			messages = append(messages, fmt.Sprintf("分数超出范围(0~%.2f)", *module.MaxScore))
		}

		status := "valid"
		hasError := false
		hasWarning := false
		for _, msg := range messages {
			if strings.Contains(msg, "将被跳过") || strings.Contains(msg, "将执行更新") {
				hasWarning = true
				continue
			}
			hasError = true
		}
		switch {
		case hasError:
			status = "error"
			result.InvalidRows++
		case hasWarning:
			status = "warning"
			result.WarningRows++
			result.ValidRows++
		default:
			result.ValidRows++
		}
		result.TotalRows++
		result.Rows = append(result.Rows, DirectScoreImportPreviewRow{
			RowNo:      row.RowNo,
			ObjectID:   row.ObjectID,
			ObjectName: row.ObjectName,
			Score:      roundToScale(row.Score, 6),
			Remark:     row.Remark,
			Status:     status,
			Messages:   messages,
		})
	}

	return result, nil
}

func (s *ImportExportService) ConfirmDirectScoreImport(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input DirectScoreImportConfirmInput,
	ipAddress string,
	userAgent string,
) (*DirectScoreImportConfirmResult, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ModuleID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}
	if len(input.Rows) == 0 {
		return nil, ErrImportNoValidRows
	}
	if len(input.Rows) > maxImportRows {
		return nil, ErrImportRowLimitExceeded
	}

	entries := make([]BatchDirectScoreEntry, 0, len(input.Rows))
	seen := make(map[uint]struct{}, len(input.Rows))
	for _, row := range input.Rows {
		if row.ObjectID == 0 || math.IsNaN(row.Score) || math.IsInf(row.Score, 0) {
			return nil, ErrInvalidParam
		}
		if _, exists := seen[row.ObjectID]; exists {
			return nil, ErrInvalidParam
		}
		seen[row.ObjectID] = struct{}{}
		entries = append(entries, BatchDirectScoreEntry{
			ObjectID: row.ObjectID,
			Score:    roundToScale(row.Score, 6),
			Remark:   strings.TrimSpace(row.Remark),
		})
	}

	batchResult, err := s.scoreService.BatchUpsertDirectScores(ctx, claims, operatorID, BatchDirectScoreInput{
		YearID:     input.YearID,
		PeriodCode: periodCode,
		ModuleID:   input.ModuleID,
		Overwrite:  input.Overwrite,
		Entries:    entries,
	}, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operatorID, "import", "direct_scores", nil, map[string]any{
		"event":      "import_direct_score_excel",
		"yearId":     input.YearID,
		"periodCode": periodCode,
		"moduleId":   input.ModuleID,
		"overwrite":  input.Overwrite,
		"requested":  len(input.Rows),
		"created":    batchResult.Created,
		"updated":    batchResult.Updated,
		"skipped":    batchResult.Skipped,
	}, ipAddress, userAgent))

	return &DirectScoreImportConfirmResult{
		Requested: len(input.Rows),
		Created:   batchResult.Created,
		Updated:   batchResult.Updated,
		Skipped:   batchResult.Skipped,
		Imported:  batchResult.Created + batchResult.Updated,
	}, nil
}

func (s *ImportExportService) ExportWorkbook(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	input ExportWorkbookInput,
	ipAddress string,
	userAgent string,
) (string, []byte, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || !isValidPeriodCode(periodCode) {
		return "", nil, ErrInvalidParam
	}

	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return "", nil, err
	}

	objectCategory := strings.TrimSpace(input.ObjectCategory)
	rankMap, err := s.loadRankingMap(ctx, input.YearID, periodCode, objectCategory, scope)
	if err != nil {
		return "", nil, err
	}
	summaryRows, err := s.loadSummaryRows(ctx, input.YearID, periodCode, objectCategory, scope)
	if err != nil {
		return "", nil, err
	}
	detailRows, err := s.loadDetailRows(ctx, input.YearID, periodCode, objectCategory, scope)
	if err != nil {
		return "", nil, err
	}
	voteRows, err := s.loadVoteStatRows(ctx, input.YearID, periodCode, objectCategory, scope)
	if err != nil {
		return "", nil, err
	}
	organizations, departments, employees, err := s.loadOrgRowsForExport(ctx, input.YearID, scope)
	if err != nil {
		return "", nil, err
	}

	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	if err := writeSummarySheet(file, summaryRows, rankMap); err != nil {
		return "", nil, err
	}
	if err := writeDetailSheet(file, detailRows); err != nil {
		return "", nil, err
	}
	if err := writeVoteSheet(file, voteRows); err != nil {
		return "", nil, err
	}
	if err := writeOrganizationSheets(file, organizations, departments, employees); err != nil {
		return "", nil, err
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("failed to render export workbook: %w", err)
	}
	fileName := fmt.Sprintf("assessment-export-%d-%s.xlsx", input.YearID, periodCode)
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operatorID, "export", "reports", nil, map[string]any{
		"event":          "export_assessment_workbook",
		"yearId":         input.YearID,
		"periodCode":     periodCode,
		"objectCategory": objectCategory,
		"summaryRows":    len(summaryRows),
		"detailRows":     len(detailRows),
		"voteRows":       len(voteRows),
	}, ipAddress, userAgent))

	return fileName, buffer.Bytes(), nil
}

func parseDirectScoreImportRows(fileContent []byte) ([]parsedDirectImportRow, error) {
	file, err := excelize.OpenReader(bytes.NewReader(fileContent))
	if err != nil {
		return nil, ErrImportFileInvalid
	}
	defer func() {
		_ = file.Close()
	}()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, ErrImportFileInvalid
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, ErrImportFileInvalid
	}

	parsedRows := make([]parsedDirectImportRow, 0, len(rows))
	for rowIndex := 2; rowIndex <= len(rows); rowIndex++ {
		row := rows[rowIndex-1]
		objectIDText := strings.TrimSpace(cellValue(row, 0))
		objectName := strings.TrimSpace(cellValue(row, 1))
		scoreText := strings.TrimSpace(cellValue(row, 2))
		remark := strings.TrimSpace(cellValue(row, 3))

		if objectIDText == "" && objectName == "" && scoreText == "" && remark == "" {
			continue
		}

		entry := parsedDirectImportRow{
			RowNo:      rowIndex,
			ObjectName: objectName,
			Remark:     remark,
			Errors:     make([]string, 0, 2),
		}
		objectID, err := parseUintValue(objectIDText)
		if err != nil || objectID == 0 {
			entry.Errors = append(entry.Errors, "对象ID无效")
		} else {
			entry.ObjectID = objectID
		}
		scoreValue, err := parseFloatValue(scoreText)
		if err != nil {
			entry.Errors = append(entry.Errors, "分数格式无效")
		} else {
			entry.Score = scoreValue
		}
		parsedRows = append(parsedRows, entry)
	}
	return parsedRows, nil
}

func cellValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func parseUintValue(raw string) (uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, fmt.Errorf("empty")
	}
	number, err := strconv.ParseUint(text, 10, 64)
	if err == nil {
		return uint(number), nil
	}
	floatValue, floatErr := strconv.ParseFloat(text, 64)
	if floatErr != nil || floatValue < 0 || math.Trunc(floatValue) != floatValue {
		return 0, err
	}
	return uint(floatValue), nil
}

func parseFloatValue(raw string) (float64, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(text, 64)
}

func (s *ImportExportService) loadAssessmentObjectMap(ctx context.Context, yearID uint, objectIDs []uint) (map[uint]model.AssessmentObject, error) {
	result := map[uint]model.AssessmentObject{}
	if len(objectIDs) == 0 {
		return result, nil
	}
	var rows []model.AssessmentObject
	if err := s.db.WithContext(ctx).
		Where("year_id = ? AND is_active = 1 AND id IN ?", yearID, objectIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query assessment objects for import: %w", err)
	}
	for _, item := range rows {
		result[item.ID] = item
	}
	return result, nil
}

func (s *ImportExportService) loadExistingDirectScoreObjectMap(
	ctx context.Context,
	yearID uint,
	periodCode string,
	moduleID uint,
	objectIDs []uint,
) (map[uint]struct{}, error) {
	result := map[uint]struct{}{}
	if len(objectIDs) == 0 {
		return result, nil
	}
	var existing []model.DirectScore
	if err := s.db.WithContext(ctx).
		Select("object_id").
		Where("year_id = ? AND period_code = ? AND module_id = ? AND object_id IN ?", yearID, periodCode, moduleID, objectIDs).
		Find(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to query existing direct scores: %w", err)
	}
	for _, item := range existing {
		result[item.ObjectID] = struct{}{}
	}
	return result, nil
}

func (s *ImportExportService) loadRankingMap(
	ctx context.Context,
	yearID uint,
	periodCode string,
	objectCategory string,
	scope *assessmentAccessScope,
) (map[uint]int, error) {
	query := s.db.WithContext(ctx).Model(&model.Ranking{}).
		Where("year_id = ? AND period_code = ? AND ranking_scope = ?", yearID, periodCode, "overall")
	if objectCategory != "" {
		query = query.Where("object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "object_id")

	var rows []model.Ranking
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query rankings for export: %w", err)
	}
	result := make(map[uint]int, len(rows))
	for _, item := range rows {
		result[item.ObjectID] = item.RankNo
	}
	return result, nil
}

func (s *ImportExportService) loadSummaryRows(
	ctx context.Context,
	yearID uint,
	periodCode string,
	objectCategory string,
	scope *assessmentAccessScope,
) ([]exportSummaryRow, error) {
	query := s.db.WithContext(ctx).Table("calculated_scores cs").
		Select("cs.object_id, ao.object_name, ao.object_type, ao.object_category, cs.final_score, cs.extra_points, cs.trigger_mode, cs.calculated_at").
		Joins("JOIN assessment_objects ao ON ao.id = cs.object_id").
		Where("cs.year_id = ? AND cs.period_code = ?", yearID, periodCode)
	if objectCategory != "" {
		query = query.Where("ao.object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "ao.id")

	var rows []exportSummaryRow
	if err := query.Order("cs.final_score DESC, ao.object_name ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query summary rows for export: %w", err)
	}
	return rows, nil
}

func (s *ImportExportService) loadDetailRows(
	ctx context.Context,
	yearID uint,
	periodCode string,
	objectCategory string,
	scope *assessmentAccessScope,
) ([]exportDetailRow, error) {
	query := s.db.WithContext(ctx).Table("calculated_module_scores cms").
		Select("cs.object_id, ao.object_name, cms.module_code, cms.module_key, cms.module_name, cms.raw_score, cms.weighted_score, cms.sort_order").
		Joins("JOIN calculated_scores cs ON cs.id = cms.calculated_score_id").
		Joins("JOIN assessment_objects ao ON ao.id = cs.object_id").
		Where("cs.year_id = ? AND cs.period_code = ?", yearID, periodCode)
	if objectCategory != "" {
		query = query.Where("ao.object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "ao.id")

	var rows []exportDetailRow
	if err := query.Order("cs.object_id ASC, cms.sort_order ASC, cms.id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query detail rows for export: %w", err)
	}
	return rows, nil
}

func (s *ImportExportService) loadVoteStatRows(
	ctx context.Context,
	yearID uint,
	periodCode string,
	objectCategory string,
	scope *assessmentAccessScope,
) ([]exportVoteStatRow, error) {
	query := s.db.WithContext(ctx).Table("vote_tasks vt").
		Select("sm.module_name, vg.group_name, vr.grade_option, COUNT(1) AS vote_count").
		Joins("JOIN vote_records vr ON vr.task_id = vt.id").
		Joins("JOIN vote_groups vg ON vg.id = vt.vote_group_id").
		Joins("JOIN score_modules sm ON sm.id = vg.module_id").
		Joins("JOIN assessment_objects ao ON ao.id = vt.object_id").
		Where("vt.year_id = ? AND vt.period_code = ? AND vt.status = ?", yearID, periodCode, "completed")
	if objectCategory != "" {
		query = query.Where("ao.object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "ao.id")

	var rows []exportVoteStatRow
	if err := query.
		Group("sm.module_name, vg.group_name, vr.grade_option").
		Order("sm.module_name ASC, vg.group_name ASC, vr.grade_option ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query vote statistics rows for export: %w", err)
	}
	return rows, nil
}

func (s *ImportExportService) loadOrgRowsForExport(
	ctx context.Context,
	yearID uint,
	scope *assessmentAccessScope,
) ([]model.Organization, []model.Department, []model.Employee, error) {
	if scope == nil {
		return []model.Organization{}, []model.Department{}, []model.Employee{}, nil
	}
	if scope.unrestricted {
		organizations := make([]model.Organization, 0)
		departments := make([]model.Department, 0)
		employees := make([]model.Employee, 0)
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id ASC").Find(&organizations).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query organizations for export: %w", err)
		}
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id ASC").Find(&departments).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query departments for export: %w", err)
		}
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id ASC").Find(&employees).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query employees for export: %w", err)
		}
		return organizations, departments, employees, nil
	}

	readableObjectIDs := setToSortedUintSlice(scope.readableObjectIDs)
	if len(readableObjectIDs) == 0 {
		return []model.Organization{}, []model.Department{}, []model.Employee{}, nil
	}

	var objects []model.AssessmentObject
	if err := s.db.WithContext(ctx).
		Where("year_id = ? AND id IN ?", yearID, readableObjectIDs).
		Find(&objects).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query scoped assessment objects for export: %w", err)
	}

	orgIDSet := map[uint]struct{}{}
	deptIDSet := map[uint]struct{}{}
	empIDSet := map[uint]struct{}{}
	for _, item := range objects {
		switch item.TargetType {
		case "organization", "leadership_team":
			orgIDSet[item.TargetID] = struct{}{}
		case "department":
			deptIDSet[item.TargetID] = struct{}{}
		case "employee":
			empIDSet[item.TargetID] = struct{}{}
		}
	}

	departmentIDs := setToSortedUintSlice(deptIDSet)
	if len(departmentIDs) > 0 {
		var departments []model.Department
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND id IN ?", departmentIDs).Find(&departments).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped departments: %w", err)
		}
		for _, item := range departments {
			orgIDSet[item.OrganizationID] = struct{}{}
		}
	}

	employeeIDs := setToSortedUintSlice(empIDSet)
	if len(employeeIDs) > 0 {
		var employees []model.Employee
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND id IN ?", employeeIDs).Find(&employees).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped employees: %w", err)
		}
		for _, item := range employees {
			orgIDSet[item.OrganizationID] = struct{}{}
			if item.DepartmentID != nil {
				deptIDSet[*item.DepartmentID] = struct{}{}
			}
		}
	}

	orgIDs := setToSortedUintSlice(orgIDSet)
	if len(orgIDs) > 0 {
		var departments []model.Department
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND organization_id IN ?", orgIDs).Find(&departments).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query organization scoped departments: %w", err)
		}
		for _, item := range departments {
			deptIDSet[item.ID] = struct{}{}
		}
	}

	deptIDs := setToSortedUintSlice(deptIDSet)
	if len(orgIDs) > 0 || len(deptIDs) > 0 {
		query := s.db.WithContext(ctx).Model(&model.Employee{}).Where("deleted_at IS NULL")
		switch {
		case len(orgIDs) > 0 && len(deptIDs) > 0:
			query = query.Where("organization_id IN ? OR department_id IN ?", orgIDs, deptIDs)
		case len(orgIDs) > 0:
			query = query.Where("organization_id IN ?", orgIDs)
		default:
			query = query.Where("department_id IN ?", deptIDs)
		}
		var employees []model.Employee
		if err := query.Find(&employees).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped employees by org/dept: %w", err)
		}
		for _, item := range employees {
			empIDSet[item.ID] = struct{}{}
			orgIDSet[item.OrganizationID] = struct{}{}
			if item.DepartmentID != nil {
				deptIDSet[*item.DepartmentID] = struct{}{}
			}
		}
	}

	orgIDs = setToSortedUintSlice(orgIDSet)
	deptIDs = setToSortedUintSlice(deptIDSet)
	employeeIDs = setToSortedUintSlice(empIDSet)

	organizations := make([]model.Organization, 0)
	departments := make([]model.Department, 0)
	employees := make([]model.Employee, 0)
	if len(orgIDs) > 0 {
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND id IN ?", orgIDs).Order("id ASC").Find(&organizations).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped organizations for export: %w", err)
		}
	}
	if len(deptIDs) > 0 {
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND id IN ?", deptIDs).Order("id ASC").Find(&departments).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped departments for export: %w", err)
		}
	}
	if len(employeeIDs) > 0 {
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL AND id IN ?", employeeIDs).Order("id ASC").Find(&employees).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("failed to query scoped employees for export: %w", err)
		}
	}
	return organizations, departments, employees, nil
}

func buildDirectScoreTemplate() (string, []byte, error) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	sheet := "直接录入分数"
	file.SetSheetName("Sheet1", sheet)
	if err := writeTableSheet(file, sheet, []string{"考核对象ID*", "考核对象名称(参考)", "分数*", "备注"}, [][]any{
		{1001, "示例对象A", 88.5, "可选备注"},
		{1002, "示例对象B", 92.0, ""},
	}); err != nil {
		return "", nil, err
	}

	notes := "填写说明"
	file.NewSheet(notes)
	_ = file.SetCellValue(notes, "A1", "1. 请保持表头不变，从第2行开始填写数据。")
	_ = file.SetCellValue(notes, "A2", "2. 对象ID必须是当前年度内已激活的考核对象ID。")
	_ = file.SetCellValue(notes, "A3", "3. 分数需在模块配置范围内，支持最多6位小数。")
	_ = file.SetCellValue(notes, "A4", "4. 导入前建议先执行“校验预览”。")
	_ = file.SetColWidth(notes, "A", "A", 90)
	sheetIndex, _ := file.GetSheetIndex(sheet)
	file.SetActiveSheet(sheetIndex)

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("failed to build direct score template: %w", err)
	}
	return "direct-score-import-template.xlsx", buffer.Bytes(), nil
}

func buildOrganizationTemplate() (string, []byte, error) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	sheet := "组织导入"
	file.SetSheetName("Sheet1", sheet)
	if err := writeTableSheet(file, sheet, []string{"单位名称*", "单位类型*(group/company)", "上级单位ID", "排序号", "状态(active/inactive)"}, [][]any{
		{"示例集团", "group", "", 1, "active"},
		{"示例权属企业", "company", 1, 10, "active"},
	}); err != nil {
		return "", nil, err
	}

	notes := "填写说明"
	file.NewSheet(notes)
	_ = file.SetCellValue(notes, "A1", "当前版本提供模板下载，组织导入能力可在后续迭代接入。")
	_ = file.SetColWidth(notes, "A", "A", 70)
	sheetIndex, _ := file.GetSheetIndex(sheet)
	file.SetActiveSheet(sheetIndex)

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("failed to build organization template: %w", err)
	}
	return "organization-import-template.xlsx", buffer.Bytes(), nil
}

func buildEmployeeTemplate() (string, []byte, error) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	sheet := "人员导入"
	file.SetSheetName("Sheet1", sheet)
	if err := writeTableSheet(file, sheet, []string{"姓名*", "单位ID*", "部门ID", "职级ID*", "岗位名称", "入职日期(YYYY-MM-DD)", "状态(active/inactive)"}, [][]any{
		{"张三", 1, 11, 1, "综合管理岗", "2025-01-15", "active"},
		{"李四", 2, "", 2, "项目管理岗", "2024-11-01", "active"},
	}); err != nil {
		return "", nil, err
	}

	notes := "填写说明"
	file.NewSheet(notes)
	_ = file.SetCellValue(notes, "A1", "当前版本提供模板下载，人员导入能力可在后续迭代接入。")
	_ = file.SetColWidth(notes, "A", "A", 70)
	sheetIndex, _ := file.GetSheetIndex(sheet)
	file.SetActiveSheet(sheetIndex)

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("failed to build employee template: %w", err)
	}
	return "employee-import-template.xlsx", buffer.Bytes(), nil
}

func writeSummarySheet(file *excelize.File, rows []exportSummaryRow, rankMap map[uint]int) error {
	sheet := "结果汇总"
	file.NewSheet(sheet)
	tableRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []any{
			rankMap[row.ObjectID],
			row.ObjectID,
			row.ObjectName,
			row.ObjectType,
			row.ObjectCategory,
			roundToScale(row.FinalScore, 6),
			roundToScale(row.ExtraPoints, 6),
			row.TriggerMode,
			formatUnixTime(row.CalculatedAt),
		})
	}
	return writeTableSheet(file, sheet, []string{
		"排名", "对象ID", "对象名称", "对象类型", "对象分类", "总分", "加减分", "触发方式", "计算时间",
	}, tableRows)
}

func writeDetailSheet(file *excelize.File, rows []exportDetailRow) error {
	sheet := "考核明细"
	file.NewSheet(sheet)
	tableRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []any{
			row.ObjectID,
			row.ObjectName,
			row.ModuleCode,
			row.ModuleKey,
			row.ModuleName,
			roundToScale(row.RawScore, 6),
			roundToScale(row.WeightedScore, 6),
			row.SortOrder,
		})
	}
	return writeTableSheet(file, sheet, []string{
		"对象ID", "对象名称", "模块编码", "模块Key", "模块名称", "原始分", "加权分", "模块排序",
	}, tableRows)
}

func writeVoteSheet(file *excelize.File, rows []exportVoteStatRow) error {
	sheet := "投票统计"
	file.NewSheet(sheet)
	tableRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []any{
			row.ModuleName,
			row.GroupName,
			row.GradeOption,
			row.VoteCount,
		})
	}
	return writeTableSheet(file, sheet, []string{
		"模块名称", "投票组", "等级选项", "票数",
	}, tableRows)
}

func writeOrganizationSheets(
	file *excelize.File,
	organizations []model.Organization,
	departments []model.Department,
	employees []model.Employee,
) error {
	orgMap := make(map[uint]model.Organization, len(organizations))
	for _, item := range organizations {
		orgMap[item.ID] = item
	}
	deptMap := make(map[uint]model.Department, len(departments))
	for _, item := range departments {
		deptMap[item.ID] = item
	}

	orgRows := make([][]any, 0, len(organizations))
	for _, item := range organizations {
		parentName := ""
		if item.ParentID != nil {
			parentName = orgMap[*item.ParentID].OrgName
		}
		orgRows = append(orgRows, []any{
			item.ID,
			item.OrgName,
			item.OrgType,
			valueOrZero(item.ParentID),
			parentName,
			item.Status,
		})
	}
	if err := writeTableSheet(file, "组织-单位", []string{
		"单位ID", "单位名称", "单位类型", "上级单位ID", "上级单位名称", "状态",
	}, orgRows); err != nil {
		return err
	}

	deptRows := make([][]any, 0, len(departments))
	for _, item := range departments {
		parentName := ""
		if item.ParentDeptID != nil {
			parentName = deptMap[*item.ParentDeptID].DeptName
		}
		deptRows = append(deptRows, []any{
			item.ID,
			item.DeptName,
			item.OrganizationID,
			orgMap[item.OrganizationID].OrgName,
			valueOrZero(item.ParentDeptID),
			parentName,
			item.Status,
		})
	}
	if err := writeTableSheet(file, "组织-部门", []string{
		"部门ID", "部门名称", "单位ID", "单位名称", "上级部门ID", "上级部门名称", "状态",
	}, deptRows); err != nil {
		return err
	}

	employeeRows := make([][]any, 0, len(employees))
	for _, item := range employees {
		deptID := uint(0)
		deptName := ""
		if item.DepartmentID != nil {
			deptID = *item.DepartmentID
			deptName = deptMap[*item.DepartmentID].DeptName
		}
		hireDate := ""
		if item.HireDate != nil {
			hireDate = item.HireDate.Format("2006-01-02")
		}
		employeeRows = append(employeeRows, []any{
			item.ID,
			item.EmpName,
			item.OrganizationID,
			orgMap[item.OrganizationID].OrgName,
			deptID,
			deptName,
			item.PositionLevelID,
			item.PositionTitle,
			hireDate,
			item.Status,
		})
	}
	return writeTableSheet(file, "组织-人员", []string{
		"人员ID", "姓名", "单位ID", "单位名称", "部门ID", "部门名称", "职级ID", "岗位", "入职日期", "状态",
	}, employeeRows)
}

func writeTableSheet(file *excelize.File, sheet string, headers []string, rows [][]any) error {
	sheetIndex, indexErr := file.GetSheetIndex(sheet)
	if indexErr != nil || sheetIndex == -1 {
		file.NewSheet(sheet)
	}
	headerStyle, _ := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D9E2F3"},
			Pattern: 1,
		},
	})

	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return fmt.Errorf("failed to set header value: %w", err)
		}
		if err := file.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return fmt.Errorf("failed to set header style: %w", err)
		}
		col, _ := excelize.ColumnNumberToName(index + 1)
		_ = file.SetColWidth(sheet, col, col, 18)
	}

	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return fmt.Errorf("failed to set table value: %w", err)
			}
		}
	}
	return nil
}

func valueOrZero(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func formatUnixTime(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}
