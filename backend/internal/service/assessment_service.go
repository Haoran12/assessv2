package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type AssessmentService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type CreateAssessmentYearInput struct {
	Year           int
	Description    string
	StartDate      *time.Time
	EndDate        *time.Time
	CopyFromYearID *uint
}

type CreateAssessmentYearResult struct {
	Year         model.AssessmentYear     `json:"year"`
	Periods      []model.AssessmentPeriod `json:"periods"`
	ObjectsCount int                      `json:"objectsCount"`
}

func NewAssessmentService(db *gorm.DB, auditRepo *repository.AuditRepository) *AssessmentService {
	return &AssessmentService{db: db, auditRepo: auditRepo}
}

func (s *AssessmentService) ListYears(ctx context.Context) ([]model.AssessmentYear, error) {
	var items []model.AssessmentYear
	if err := s.db.WithContext(ctx).Order("year DESC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment years: %w", err)
	}
	for idx := range items {
		items[idx].Status = normalizeYearStatus(items[idx].Status)
	}
	return items, nil
}

func (s *AssessmentService) CreateYear(ctx context.Context, claims *auth.Claims, operatorID uint, input CreateAssessmentYearInput, ipAddress string, userAgent string) (*CreateAssessmentYearResult, error) {
	if err := requireCreateAssessmentYearScope(ctx, s.db, claims); err != nil {
		return nil, err
	}
	if input.Year < 2000 || input.Year > 9999 {
		return nil, ErrInvalidParam
	}
	if input.StartDate != nil && input.EndDate != nil && input.StartDate.After(*input.EndDate) {
		return nil, ErrInvalidParam
	}
	if err := ensureAssessmentYearDataDirectory(input.Year); err != nil {
		return nil, err
	}
	var existingCount int64
	if err := s.db.WithContext(ctx).Model(&model.AssessmentYear{}).Where("year = ?", input.Year).Count(&existingCount).Error; err != nil {
		return nil, fmt.Errorf("failed to verify assessment year uniqueness: %w", err)
	}
	if existingCount > 0 {
		return nil, ErrYearAlreadyExists
	}

	operator := operatorID
	result := &CreateAssessmentYearResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef := resolveBusinessWriteOperatorRefTx(tx, operator)
		year, err := createAssessmentYearTx(tx, input, operatorRef)
		if err != nil {
			if isUniqueConstraintError(err) {
				return ErrYearAlreadyExists
			}
			return fmt.Errorf("failed to create assessment year: %w", err)
		}

		templates, err := s.loadPeriodTemplatesTx(tx)
		if err != nil {
			return fmt.Errorf("failed to load assessment period templates: %w", err)
		}
		periods, err := buildPeriodsFromTemplates(year.ID, year.Year, operatorRef, templates)
		if err != nil {
			return err
		}
		if err := tx.Create(&periods).Error; err != nil {
			return fmt.Errorf("failed to create assessment periods: %w", err)
		}

		objectsCount := 0
		if input.CopyFromYearID != nil && *input.CopyFromYearID > 0 {
			copied, err := s.copyAssessmentObjects(tx, *input.CopyFromYearID, year.ID, operatorRef)
			if err != nil {
				return err
			}
			objectsCount = copied
		} else {
			generated, err := s.generateAssessmentObjects(tx, year.ID, operatorRef)
			if err != nil {
				return err
			}
			objectsCount = generated
		}

		result.Year = *year
		result.Periods = periods
		result.ObjectsCount = objectsCount
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := result.Year.ID
	copyFrom := uint(0)
	if input.CopyFromYearID != nil {
		copyFrom = *input.CopyFromYearID
	}
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "assessment_years", &targetID, map[string]any{
		"event":        "create_assessment_year",
		"year":         result.Year.Year,
		"copyFromYear": copyFrom,
		"objectsCount": result.ObjectsCount,
	}, ipAddress, userAgent))

	return result, nil
}

func (s *AssessmentService) UpdateYearStatus(ctx context.Context, claims *auth.Claims, operatorID, yearID uint, status string, ipAddress string, userAgent string) (*model.AssessmentYear, error) {
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}
	next := normalizeYearStatus(status)
	if !isValidYearStatus(next) {
		return nil, ErrInvalidYearStatus
	}

	var year model.AssessmentYear
	if err := s.db.WithContext(ctx).Where("id = ?", yearID).First(&year).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrYearNotFound
		}
		return nil, fmt.Errorf("failed to query assessment year: %w", err)
	}
	current := normalizeYearStatus(year.Status)
	if current != next && !canTransitionYearStatus(current, next) {
		return nil, ErrInvalidYearTransition
	}
	if current == next {
		year.Status = current
		return &year, nil
	}

	operator := operatorID
	if err := s.db.WithContext(ctx).Model(&model.AssessmentYear{}).Where("id = ?", yearID).Updates(map[string]any{"status": next, "updated_by": &operator, "updated_at": time.Now().Unix()}).Error; err != nil {
		return nil, fmt.Errorf("failed to update assessment year status: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ?", yearID).First(&year).Error; err != nil {
		return nil, fmt.Errorf("failed to reload assessment year: %w", err)
	}

	targetID := year.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "assessment_years", &targetID, map[string]any{"event": "update_assessment_year_status", "status": next}, ipAddress, userAgent))
	return &year, nil
}

func (s *AssessmentService) ListPeriods(ctx context.Context, yearID uint) ([]model.AssessmentPeriod, error) {
	if yearID == 0 {
		return nil, ErrInvalidParam
	}
	var items []model.AssessmentPeriod
	if err := s.db.WithContext(ctx).Where("year_id = ?", yearID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list periods: %w", err)
	}
	for idx := range items {
		items[idx].Status = normalizePeriodStatus(items[idx].Status)
	}
	return items, nil
}

func (s *AssessmentService) UpdatePeriodStatus(ctx context.Context, claims *auth.Claims, operatorID, periodID uint, status string, ipAddress string, userAgent string) (*model.AssessmentPeriod, error) {
	if err := requireRootOrAssessmentAdminClaims(claims); err != nil {
		return nil, err
	}
	next := normalizePeriodStatus(status)
	if !isValidPeriodStatus(next) {
		return nil, ErrInvalidPeriodStatus
	}

	var period model.AssessmentPeriod
	if err := s.db.WithContext(ctx).Where("id = ?", periodID).First(&period).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrPeriodNotFound
		}
		return nil, fmt.Errorf("failed to query assessment period: %w", err)
	}
	current := normalizePeriodStatus(period.Status)
	if current != next && !canTransitionPeriodStatus(current, next) {
		return nil, ErrInvalidPeriodTransition
	}
	if current == next {
		period.Status = current
		return &period, nil
	}

	operator := operatorID
	if err := s.db.WithContext(ctx).Model(&model.AssessmentPeriod{}).Where("id = ?", periodID).Updates(map[string]any{"status": next, "updated_by": &operator, "updated_at": time.Now().Unix()}).Error; err != nil {
		return nil, fmt.Errorf("failed to update assessment period status: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ?", periodID).First(&period).Error; err != nil {
		return nil, fmt.Errorf("failed to reload assessment period: %w", err)
	}

	targetID := period.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "assessment_periods", &targetID, map[string]any{"event": "update_assessment_period_status", "status": next}, ipAddress, userAgent))
	return &period, nil
}

func (s *AssessmentService) ListObjects(ctx context.Context, claims *auth.Claims, yearID uint) ([]model.AssessmentObject, error) {
	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Model(&model.AssessmentObject{}).Where("year_id = ?", yearID)
	query = scope.applyReadableObjectFilter(query, "id")

	var items []model.AssessmentObject
	if err := query.Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment objects: %w", err)
	}
	return items, nil
}

func defaultPeriods(yearID uint, operatorID *uint) []model.AssessmentPeriod {
	base := []struct {
		Code string
		Name string
	}{
		{Code: "Q1", Name: "第一季度"},
		{Code: "Q2", Name: "第二季度"},
		{Code: "Q3", Name: "第三季度"},
		{Code: "Q4", Name: "第四季度"},
		{Code: "YEAR_END", Name: "年终考核"},
	}
	items := make([]model.AssessmentPeriod, 0, len(base))
	for _, item := range base {
		items = append(items, model.AssessmentPeriod{YearID: yearID, PeriodCode: item.Code, PeriodName: item.Name, Status: assessmentStatusPreparing, CreatedBy: operatorID, UpdatedBy: operatorID})
	}
	return items
}

func (s *AssessmentService) copyAssessmentObjects(tx *gorm.DB, sourceYearID, targetYearID uint, operatorID *uint) (int, error) {
	var years []model.AssessmentYear
	if err := tx.Where("id IN ?", []uint{sourceYearID, targetYearID}).Find(&years).Error; err != nil {
		return 0, fmt.Errorf("failed to verify year records: %w", err)
	}
	if len(years) < 2 {
		return 0, ErrYearNotFound
	}

	var source []model.AssessmentObject
	if err := tx.Where("year_id = ? AND is_active = 1", sourceYearID).Order("id ASC").Find(&source).Error; err != nil {
		return 0, fmt.Errorf("failed to query source assessment objects: %w", err)
	}
	if len(source) == 0 {
		return 0, nil
	}

	idMap := map[uint]uint{}
	parentMap := map[uint]*uint{}
	count := 0
	for _, item := range source {
		active, err := s.isTargetActive(tx, item.TargetType, item.TargetID)
		if err != nil {
			return 0, err
		}
		if !active {
			continue
		}

		record := model.AssessmentObject{YearID: targetYearID, ObjectType: item.ObjectType, ObjectCategory: item.ObjectCategory, TargetID: item.TargetID, TargetType: item.TargetType, ObjectName: item.ObjectName, ParentObjectID: nil, IsActive: item.IsActive, CreatedBy: operatorID, UpdatedBy: operatorID}
		if err := tx.Create(&record).Error; err != nil {
			if isUniqueConstraintError(err) {
				continue
			}
			return 0, fmt.Errorf("failed to copy assessment object target=%s:%d: %w", item.TargetType, item.TargetID, err)
		}
		idMap[item.ID] = record.ID
		parentMap[record.ID] = item.ParentObjectID
		count++
	}

	for recordID, oldParentID := range parentMap {
		if oldParentID == nil {
			continue
		}
		newParentID, ok := idMap[*oldParentID]
		if !ok {
			continue
		}
		if err := tx.Model(&model.AssessmentObject{}).Where("id = ?", recordID).Update("parent_object_id", newParentID).Error; err != nil {
			return 0, fmt.Errorf("failed to update copied object parent: %w", err)
		}
	}

	return count, nil
}

func (s *AssessmentService) generateAssessmentObjects(tx *gorm.DB, yearID uint, operatorID *uint) (int, error) {
	count := 0
	teamKeyToObjectID := map[string]uint{}

	var organizations []model.Organization
	if err := tx.Where("org_type IN ? AND status = ? AND deleted_at IS NULL", []string{"group", "company"}, "active").Order("id ASC").Find(&organizations).Error; err != nil {
		return 0, fmt.Errorf("failed to query active organizations: %w", err)
	}
	for _, item := range organizations {
		mainCategory := TeamCategorySubsidiaryCompany
		leadershipCategory := TeamCategorySubsidiaryCompanyLeadership
		if item.OrgType == "group" {
			mainCategory = TeamCategoryGroup
			leadershipCategory = TeamCategoryGroupLeadership
		}

		mainRecord := model.AssessmentObject{
			YearID:         yearID,
			ObjectType:     ObjectTypeTeam,
			ObjectCategory: mainCategory,
			TargetID:       item.ID,
			TargetType:     "organization",
			ObjectName:     item.OrgName,
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		mainObjectID, created, err := createAssessmentObjectTx(tx, mainRecord)
		if err != nil {
			return 0, fmt.Errorf("failed to generate organization object: %w", err)
		}
		if created {
			count++
		}
		teamKeyToObjectID[teamKey("organization", item.ID)] = mainObjectID

		leadershipRecord := model.AssessmentObject{
			YearID:         yearID,
			ObjectType:     ObjectTypeTeam,
			ObjectCategory: leadershipCategory,
			TargetID:       item.ID,
			TargetType:     "leadership_team",
			ObjectName:     fmt.Sprintf("%s领导班子", item.OrgName),
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		leadershipObjectID, created, err := createAssessmentObjectTx(tx, leadershipRecord)
		if err != nil {
			return 0, fmt.Errorf("failed to generate leadership team object: %w", err)
		}
		if created {
			count++
		}
		teamKeyToObjectID[teamKey("leadership_team", item.ID)] = leadershipObjectID
	}

	var departments []struct {
		ID             uint
		DeptName       string
		OrganizationID uint
		OrgType        string
	}
	if err := tx.Table("departments d").
		Select("d.id, d.dept_name, d.organization_id, o.org_type").
		Joins("JOIN organizations o ON o.id = d.organization_id").
		Where("d.deleted_at IS NULL AND d.status = ? AND o.deleted_at IS NULL AND o.status = ?", "active", "active").
		Order("d.id ASC").
		Scan(&departments).Error; err != nil {
		return 0, fmt.Errorf("failed to query active departments: %w", err)
	}
	for _, item := range departments {
		category := TeamCategorySubsidiaryCompanyDepartment
		if item.OrgType == "group" {
			category = TeamCategoryGroupDepartment
		}
		record := model.AssessmentObject{
			YearID:         yearID,
			ObjectType:     ObjectTypeTeam,
			ObjectCategory: category,
			TargetID:       item.ID,
			TargetType:     "department",
			ObjectName:     item.DeptName,
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		objectID, created, err := createAssessmentObjectTx(tx, record)
		if err != nil {
			return 0, fmt.Errorf("failed to generate department object: %w", err)
		}
		if created {
			count++
		}
		teamKeyToObjectID[teamKey("department", item.ID)] = objectID
	}

	var employees []struct {
		ID             uint
		EmpName        string
		OrganizationID uint
		DepartmentID   *uint
		OrgType        string
		LevelCode      string
	}
	if err := tx.Table("employees e").
		Select("e.id, e.emp_name, e.organization_id, e.department_id, o.org_type, p.level_code").
		Joins("JOIN organizations o ON o.id = e.organization_id").
		Joins("JOIN position_levels p ON p.id = e.position_level_id").
		Joins("LEFT JOIN departments d ON d.id = e.department_id").
		Where("e.deleted_at IS NULL AND e.status = ? AND o.deleted_at IS NULL AND o.status = ?", "active", "active").
		Where("e.department_id IS NULL OR (d.deleted_at IS NULL AND d.status = 'active')").
		Order("e.id ASC").
		Scan(&employees).Error; err != nil {
		return 0, fmt.Errorf("failed to query active employees: %w", err)
	}
	for _, item := range employees {
		category := normalizeEmployeeCategory(item.LevelCode)
		var parentObjectID *uint
		if item.DepartmentID != nil {
			if objectID, ok := teamKeyToObjectID[teamKey("department", *item.DepartmentID)]; ok {
				parentObjectID = uintPtr(objectID)
			}
		}
		if parentObjectID == nil {
			if objectID, ok := teamKeyToObjectID[teamKey("organization", item.OrganizationID)]; ok {
				parentObjectID = uintPtr(objectID)
			}
		}
		record := model.AssessmentObject{
			YearID:         yearID,
			ObjectType:     ObjectTypeIndividual,
			ObjectCategory: category,
			TargetID:       item.ID,
			TargetType:     "employee",
			ObjectName:     item.EmpName,
			ParentObjectID: parentObjectID,
			IsActive:       true,
			CreatedBy:      operatorID,
			UpdatedBy:      operatorID,
		}
		_, created, err := createAssessmentObjectTx(tx, record)
		if err != nil {
			return 0, fmt.Errorf("failed to generate employee object: %w", err)
		}
		if created {
			count++
		}
	}

	return count, nil
}

func (s *AssessmentService) isTargetActive(tx *gorm.DB, targetType string, targetID uint) (bool, error) {
	var count int64
	switch targetType {
	case "organization":
		if err := tx.Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL AND status = ?", targetID, "active").Count(&count).Error; err != nil {
			return false, fmt.Errorf("failed to verify active organization target: %w", err)
		}
	case "leadership_team":
		if err := tx.Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL AND status = ?", targetID, "active").Count(&count).Error; err != nil {
			return false, fmt.Errorf("failed to verify active leadership team target: %w", err)
		}
	case "department":
		if err := tx.Table("departments d").Joins("JOIN organizations o ON o.id = d.organization_id").Where("d.id = ? AND d.deleted_at IS NULL AND d.status = 'active' AND o.deleted_at IS NULL AND o.status = 'active'", targetID).Count(&count).Error; err != nil {
			return false, fmt.Errorf("failed to verify active department target: %w", err)
		}
	case "employee":
		if err := tx.Table("employees e").Joins("JOIN organizations o ON o.id = e.organization_id").Joins("LEFT JOIN departments d ON d.id = e.department_id").Where("e.id = ? AND e.deleted_at IS NULL AND e.status = 'active' AND o.deleted_at IS NULL AND o.status = 'active'", targetID).Where("e.department_id IS NULL OR (d.deleted_at IS NULL AND d.status = 'active')").Count(&count).Error; err != nil {
			return false, fmt.Errorf("failed to verify active employee target: %w", err)
		}
	default:
		return false, nil
	}
	return count > 0, nil
}

func teamKey(targetType string, targetID uint) string {
	return fmt.Sprintf("%s:%d", targetType, targetID)
}

func normalizeEmployeeCategory(levelCode string) string {
	return normalizeIndividualCategoryFromLevelCode(levelCode)
}

func createAssessmentObjectTx(tx *gorm.DB, input model.AssessmentObject) (uint, bool, error) {
	if err := tx.Create(&input).Error; err != nil {
		if isUniqueConstraintError(err) {
			var existing model.AssessmentObject
			findErr := tx.Where(
				"year_id = ? AND target_type = ? AND target_id = ?",
				input.YearID,
				input.TargetType,
				input.TargetID,
			).First(&existing).Error
			if findErr != nil {
				return 0, false, fmt.Errorf("failed to load existing assessment object: %w", findErr)
			}
			return existing.ID, false, nil
		}
		return 0, false, err
	}
	return input.ID, true, nil
}

func isValidYearStatus(status string) bool {
	normalized := normalizeYearStatus(status)
	switch normalized {
	case assessmentStatusPreparing, assessmentStatusActive, assessmentStatusCompleted:
		return true
	default:
		return false
	}
}

func isValidPeriodStatus(status string) bool {
	normalized := normalizePeriodStatus(status)
	switch normalized {
	case assessmentStatusPreparing, assessmentStatusActive, assessmentStatusCompleted:
		return true
	default:
		return false
	}
}

func canTransitionYearStatus(current, next string) bool {
	if current == next {
		return true
	}
	return isValidYearStatus(current) && isValidYearStatus(next)
}

func canTransitionPeriodStatus(current, next string) bool {
	if current == next {
		return true
	}
	return isValidPeriodStatus(current) && isValidPeriodStatus(next)
}

func resolveBusinessWriteOperatorRefTx(tx *gorm.DB, operatorID uint) *uint {
	if operatorID == 0 || tx == nil {
		return nil
	}
	if !tx.Migrator().HasTable("users") {
		return nil
	}
	var count int64
	if err := tx.Table("users").Where("id = ?", operatorID).Count(&count).Error; err != nil {
		return nil
	}
	if count == 0 {
		return nil
	}
	value := operatorID
	return &value
}

func createAssessmentYearTx(tx *gorm.DB, input CreateAssessmentYearInput, operatorID *uint) (*model.AssessmentYear, error) {
	year := &model.AssessmentYear{
		Year:        input.Year,
		Status:      assessmentStatusPreparing,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
		Description: strings.TrimSpace(input.Description),
		CreatedBy:   operatorID,
		UpdatedBy:   operatorID,
	}

	if !tx.Migrator().HasColumn("assessment_years", "year_name") {
		if err := tx.Create(year).Error; err != nil {
			return nil, err
		}
		return year, nil
	}

	// Backward compatibility for historical schemas where `year_name` still exists and is NOT NULL.
	now := time.Now().Unix()
	legacyPayload := map[string]any{
		"year":        input.Year,
		"year_name":   strconv.Itoa(input.Year),
		"status":      assessmentStatusPreparing,
		"start_date":  input.StartDate,
		"end_date":    input.EndDate,
		"description": strings.TrimSpace(input.Description),
		"created_by":  operatorID,
		"updated_by":  operatorID,
		"created_at":  now,
		"updated_at":  now,
	}
	if tx.Migrator().HasColumn("assessment_years", "permission_mode") {
		legacyPayload["permission_mode"] = uint16(420)
	}

	if err := tx.Table("assessment_years").Create(legacyPayload).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("year = ?", input.Year).First(year).Error; err != nil {
		return nil, err
	}
	return year, nil
}

func ensureAssessmentYearDataDirectory(year int) error {
	dataRoot := strings.TrimSpace(os.Getenv("ASSESS_DATA_ROOT"))
	if dataRoot == "" {
		return nil
	}
	yearDir := filepath.Join(dataRoot, strconv.Itoa(year))
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		return fmt.Errorf("failed to create assessment year data dir: %w", err)
	}
	return nil
}
