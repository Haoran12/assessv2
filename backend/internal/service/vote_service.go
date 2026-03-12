package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VoteService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type GenerateVoteTasksInput struct {
	YearID     uint
	PeriodCode string
	ModuleID   uint
	ObjectIDs  []uint
}

type GenerateVoteTasksResult struct {
	Created     int `json:"created"`
	Skipped     int `json:"skipped"`
	GroupCount  int `json:"groupCount"`
	ObjectCount int `json:"objectCount"`
	VoterCount  int `json:"voterCount"`
}

type ListVoteTaskFilter struct {
	YearID     *uint
	PeriodCode string
	ModuleID   *uint
	ObjectID   *uint
	VoterID    *uint
	Status     string
}

type VoteTaskListItem struct {
	model.VoteTask
	ModuleID    uint   `json:"moduleId"`
	ModuleName  string `json:"moduleName"`
	GroupCode   string `json:"groupCode"`
	GroupName   string `json:"groupName"`
	GradeOption string `json:"gradeOption,omitempty"`
	Remark      string `json:"remark,omitempty"`
	VotedAt     *int64 `json:"votedAt,omitempty"`
}

type VoteRecordInput struct {
	GradeOption string
	Remark      string
}

type VoteRecordResult struct {
	Task   model.VoteTask   `json:"task"`
	Record model.VoteRecord `json:"record"`
}

type VoteStatisticsFilter struct {
	YearID     uint
	PeriodCode string
	ModuleID   uint
	ObjectID   *uint
}

type VoteGroupStatistics struct {
	VoteGroupID    uint           `json:"voteGroupId"`
	GroupCode      string         `json:"groupCode"`
	GroupName      string         `json:"groupName"`
	TotalTasks     int            `json:"totalTasks"`
	CompletedTasks int            `json:"completedTasks"`
	PendingTasks   int            `json:"pendingTasks"`
	ExpiredTasks   int            `json:"expiredTasks"`
	GradeCounts    map[string]int `json:"gradeCounts"`
}

type VoteStatistics struct {
	TotalTasks      int                   `json:"totalTasks"`
	CompletedTasks  int                   `json:"completedTasks"`
	PendingTasks    int                   `json:"pendingTasks"`
	ExpiredTasks    int                   `json:"expiredTasks"`
	CompletionRate  float64               `json:"completionRate"`
	GroupStatistics []VoteGroupStatistics `json:"groupStatistics"`
}

type voteScope struct {
	UserIDs         []uint   `json:"user_ids"`
	OrganizationIDs []uint   `json:"organization_ids"`
	RoleCodes       []string `json:"role_codes"`
}

func NewVoteService(db *gorm.DB, auditRepo *repository.AuditRepository) *VoteService {
	return &VoteService{db: db, auditRepo: auditRepo}
}

func (s *VoteService) GenerateVoteTasks(
	ctx context.Context,
	operatorID uint,
	input GenerateVoteTasksInput,
	ipAddress string,
	userAgent string,
) (*GenerateVoteTasksResult, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ModuleID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}

	operator := operatorID
	now := time.Now().Unix()
	result := &GenerateVoteTasksResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePeriodWritableTx(tx, input.YearID, periodCode); err != nil {
			return err
		}
		if _, err := loadModuleByPeriodTx(tx, input.ModuleID, "vote", input.YearID, periodCode); err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrInvalidVoteModule
			}
			return fmt.Errorf("failed to query vote module: %w", err)
		}

		var groups []model.VoteGroup
		if err := tx.Where("module_id = ? AND is_active = 1", input.ModuleID).Order("sort_order ASC, id ASC").Find(&groups).Error; err != nil {
			return fmt.Errorf("failed to query vote groups: %w", err)
		}
		if len(groups) == 0 {
			return ErrInvalidParam
		}
		result.GroupCount = len(groups)

		objects, err := resolveVoteTaskObjectsTx(tx, input.YearID, input.ObjectIDs)
		if err != nil {
			return err
		}
		if len(objects) == 0 {
			return ErrInvalidParam
		}
		result.ObjectCount = len(objects)

		voterSet := make(map[uint]struct{})
		for _, group := range groups {
			voters, err := resolveVoteGroupVotersTx(tx, group)
			if err != nil {
				return err
			}
			if len(voters) == 0 {
				continue
			}
			for _, voterID := range voters {
				voterSet[voterID] = struct{}{}
			}
			for _, object := range objects {
				for _, voterID := range voters {
					task := model.VoteTask{
						YearID:      input.YearID,
						PeriodCode:  periodCode,
						VoteGroupID: group.ID,
						ObjectID:    object.ID,
						VoterID:     voterID,
						Status:      "pending",
						CreatedBy:   &operator,
						CreatedAt:   now,
						UpdatedAt:   now,
					}
					res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&task)
					if res.Error != nil {
						return fmt.Errorf("failed to create vote task: %w", res.Error)
					}
					if res.RowsAffected > 0 {
						result.Created++
					} else {
						result.Skipped++
					}
				}
			}
		}
		result.VoterCount = len(voterSet)
		return nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "vote_tasks", nil, map[string]any{
		"event":       "generate_vote_tasks",
		"yearId":      input.YearID,
		"periodCode":  periodCode,
		"moduleId":    input.ModuleID,
		"created":     result.Created,
		"skipped":     result.Skipped,
		"groupCount":  result.GroupCount,
		"objectCount": result.ObjectCount,
		"voterCount":  result.VoterCount,
	}, ipAddress, userAgent))
	return result, nil
}

func (s *VoteService) ListVoteTasks(ctx context.Context, filter ListVoteTaskFilter) ([]VoteTaskListItem, error) {
	status := strings.ToLower(strings.TrimSpace(filter.Status))
	if status != "" {
		if _, ok := voteTaskStatusSet[status]; !ok {
			return nil, ErrInvalidVoteTaskStatus
		}
	}

	type voteTaskRow struct {
		ID          uint
		YearID      uint
		PeriodCode  string
		VoteGroupID uint
		ObjectID    uint
		VoterID     uint
		Status      string
		CompletedAt sql.NullInt64
		CreatedBy   sql.NullInt64
		CreatedAt   int64
		UpdatedAt   int64
		ModuleID    uint
		ModuleName  string
		GroupCode   string
		GroupName   string
		GradeOption sql.NullString
		Remark      sql.NullString
		VotedAt     sql.NullInt64
	}

	query := s.db.WithContext(ctx).Table("vote_tasks vt").
		Select(
			"vt.id, vt.year_id, vt.period_code, vt.vote_group_id, vt.object_id, vt.voter_id, vt.status, vt.completed_at, vt.created_by, vt.created_at, vt.updated_at, " +
				"vg.module_id AS module_id, sm.module_name AS module_name, vg.group_code, vg.group_name, vr.grade_option, vr.remark, vr.voted_at",
		).
		Joins("JOIN vote_groups vg ON vg.id = vt.vote_group_id").
		Joins("JOIN score_modules sm ON sm.id = vg.module_id").
		Joins("LEFT JOIN vote_records vr ON vr.task_id = vt.id")
	if filter.YearID != nil {
		query = query.Where("vt.year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("vt.period_code = ?", periodCode)
	}
	if filter.ModuleID != nil {
		query = query.Where("vg.module_id = ?", *filter.ModuleID)
	}
	if filter.ObjectID != nil {
		query = query.Where("vt.object_id = ?", *filter.ObjectID)
	}
	if filter.VoterID != nil {
		query = query.Where("vt.voter_id = ?", *filter.VoterID)
	}
	if status != "" {
		query = query.Where("vt.status = ?", status)
	}

	var rows []voteTaskRow
	if err := query.Order("CASE vt.status WHEN 'pending' THEN 0 WHEN 'completed' THEN 1 ELSE 2 END ASC, vt.id DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list vote tasks: %w", err)
	}

	items := make([]VoteTaskListItem, 0, len(rows))
	for _, row := range rows {
		item := VoteTaskListItem{
			VoteTask: model.VoteTask{
				ID:          row.ID,
				YearID:      row.YearID,
				PeriodCode:  row.PeriodCode,
				VoteGroupID: row.VoteGroupID,
				ObjectID:    row.ObjectID,
				VoterID:     row.VoterID,
				Status:      row.Status,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
			},
			ModuleID:   row.ModuleID,
			ModuleName: row.ModuleName,
			GroupCode:  row.GroupCode,
			GroupName:  row.GroupName,
		}
		if row.CompletedAt.Valid {
			completedAt := row.CompletedAt.Int64
			item.CompletedAt = &completedAt
		}
		if row.CreatedBy.Valid {
			createdBy := uint(row.CreatedBy.Int64)
			item.CreatedBy = &createdBy
		}
		if row.GradeOption.Valid {
			item.GradeOption = row.GradeOption.String
		}
		if row.Remark.Valid {
			item.Remark = row.Remark.String
		}
		if row.VotedAt.Valid {
			votedAt := row.VotedAt.Int64
			item.VotedAt = &votedAt
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *VoteService) SaveVoteDraft(
	ctx context.Context,
	operatorID uint,
	isRoot bool,
	taskID uint,
	input VoteRecordInput,
	ipAddress string,
	userAgent string,
) (*VoteRecordResult, error) {
	return s.saveVoteRecord(ctx, operatorID, isRoot, taskID, input, false, ipAddress, userAgent)
}

func (s *VoteService) SubmitVote(
	ctx context.Context,
	operatorID uint,
	isRoot bool,
	taskID uint,
	input VoteRecordInput,
	ipAddress string,
	userAgent string,
) (*VoteRecordResult, error) {
	return s.saveVoteRecord(ctx, operatorID, isRoot, taskID, input, true, ipAddress, userAgent)
}

func (s *VoteService) ResetVoteTask(
	ctx context.Context,
	operatorID uint,
	taskID uint,
	ipAddress string,
	userAgent string,
) (*model.VoteTask, error) {
	if taskID == 0 {
		return nil, ErrInvalidParam
	}

	operator := operatorID
	now := time.Now().Unix()
	var task model.VoteTask
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrVoteTaskNotFound
			}
			return fmt.Errorf("failed to query vote task: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, task.YearID, task.PeriodCode); err != nil {
			return err
		}
		if err := tx.Model(&model.VoteTask{}).Where("id = ?", taskID).Updates(map[string]any{
			"status":       "pending",
			"completed_at": nil,
			"updated_at":   now,
		}).Error; err != nil {
			return fmt.Errorf("failed to reset vote task: %w", err)
		}
		if err := tx.Where("task_id = ?", taskID).Delete(&model.VoteRecord{}).Error; err != nil {
			return fmt.Errorf("failed to clear vote record: %w", err)
		}
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("failed to reload vote task: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := task.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "vote_tasks", &targetID, map[string]any{
		"event": "reset_vote_task",
	}, ipAddress, userAgent))
	return &task, nil
}

func (s *VoteService) ListVoteStatistics(ctx context.Context, filter VoteStatisticsFilter) (*VoteStatistics, error) {
	periodCode := normalizePeriodCode(filter.PeriodCode)
	if filter.YearID == 0 || filter.ModuleID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}

	type voteStatsRow struct {
		VoteGroupID uint
		GroupCode   string
		GroupName   string
		SortOrder   int
		Status      string
		GradeOption sql.NullString
	}

	query := s.db.WithContext(ctx).Table("vote_tasks vt").
		Select("vt.vote_group_id, vg.group_code, vg.group_name, vg.sort_order, vt.status, vr.grade_option").
		Joins("JOIN vote_groups vg ON vg.id = vt.vote_group_id").
		Joins("LEFT JOIN vote_records vr ON vr.task_id = vt.id").
		Where("vt.year_id = ? AND vt.period_code = ? AND vg.module_id = ?", filter.YearID, periodCode, filter.ModuleID)
	if filter.ObjectID != nil {
		query = query.Where("vt.object_id = ?", *filter.ObjectID)
	}

	var rows []voteStatsRow
	if err := query.Order("vg.sort_order ASC, vg.id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query vote statistics: %w", err)
	}

	result := &VoteStatistics{
		GroupStatistics: make([]VoteGroupStatistics, 0),
	}
	if len(rows) == 0 {
		return result, nil
	}

	type groupOrder struct {
		VoteGroupID uint
		SortOrder   int
	}
	groupStatMap := map[uint]*VoteGroupStatistics{}
	groupOrderList := make([]groupOrder, 0, 4)
	for _, row := range rows {
		result.TotalTasks++
		switch row.Status {
		case "completed":
			result.CompletedTasks++
		case "pending":
			result.PendingTasks++
		case "expired":
			result.ExpiredTasks++
		}

		groupStat, exists := groupStatMap[row.VoteGroupID]
		if !exists {
			groupStat = &VoteGroupStatistics{
				VoteGroupID: row.VoteGroupID,
				GroupCode:   row.GroupCode,
				GroupName:   row.GroupName,
				GradeCounts: map[string]int{
					"excellent": 0,
					"good":      0,
					"average":   0,
					"poor":      0,
				},
			}
			groupStatMap[row.VoteGroupID] = groupStat
			groupOrderList = append(groupOrderList, groupOrder{VoteGroupID: row.VoteGroupID, SortOrder: row.SortOrder})
		}

		groupStat.TotalTasks++
		switch row.Status {
		case "completed":
			groupStat.CompletedTasks++
		case "pending":
			groupStat.PendingTasks++
		case "expired":
			groupStat.ExpiredTasks++
		}
		if row.GradeOption.Valid {
			grade := strings.ToLower(strings.TrimSpace(row.GradeOption.String))
			if _, ok := voteGradeOptionSet[grade]; ok {
				groupStat.GradeCounts[grade]++
			}
		}
	}

	if result.TotalTasks > 0 {
		result.CompletionRate = roundToScale(float64(result.CompletedTasks)/float64(result.TotalTasks), 4)
	}
	sort.Slice(groupOrderList, func(i, j int) bool {
		if groupOrderList[i].SortOrder == groupOrderList[j].SortOrder {
			return groupOrderList[i].VoteGroupID < groupOrderList[j].VoteGroupID
		}
		return groupOrderList[i].SortOrder < groupOrderList[j].SortOrder
	})
	for _, item := range groupOrderList {
		if stat := groupStatMap[item.VoteGroupID]; stat != nil {
			result.GroupStatistics = append(result.GroupStatistics, *stat)
		}
	}
	return result, nil
}

func (s *VoteService) saveVoteRecord(
	ctx context.Context,
	operatorID uint,
	isRoot bool,
	taskID uint,
	input VoteRecordInput,
	finalSubmit bool,
	ipAddress string,
	userAgent string,
) (*VoteRecordResult, error) {
	if taskID == 0 {
		return nil, ErrInvalidParam
	}
	gradeOption := strings.ToLower(strings.TrimSpace(input.GradeOption))
	if _, ok := voteGradeOptionSet[gradeOption]; !ok {
		return nil, ErrInvalidVoteGradeOption
	}
	remark := strings.TrimSpace(input.Remark)

	operator := operatorID
	now := time.Now().Unix()
	result := &VoteRecordResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.VoteTask
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrVoteTaskNotFound
			}
			return fmt.Errorf("failed to query vote task: %w", err)
		}
		if !isRoot && task.VoterID != operatorID {
			return ErrVoteTaskForbidden
		}
		if err := ensurePeriodWritableTx(tx, task.YearID, task.PeriodCode); err != nil {
			return err
		}
		if task.Status == "completed" {
			return ErrVoteTaskLocked
		}
		if task.Status != "pending" {
			return ErrInvalidVoteTaskStatus
		}

		var record model.VoteRecord
		if err := tx.Where("task_id = ?", taskID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				record = model.VoteRecord{
					TaskID:      taskID,
					GradeOption: gradeOption,
					Remark:      remark,
					VotedAt:     now,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := tx.Create(&record).Error; err != nil {
					return fmt.Errorf("failed to create vote record: %w", err)
				}
			} else {
				return fmt.Errorf("failed to query vote record: %w", err)
			}
		} else {
			if err := tx.Model(&model.VoteRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
				"grade_option": gradeOption,
				"remark":       remark,
				"voted_at":     now,
				"updated_at":   now,
			}).Error; err != nil {
				return fmt.Errorf("failed to update vote record: %w", err)
			}
			if err := tx.Where("id = ?", record.ID).First(&record).Error; err != nil {
				return fmt.Errorf("failed to reload vote record: %w", err)
			}
		}

		taskUpdates := map[string]any{
			"updated_at": now,
		}
		if finalSubmit {
			taskUpdates["status"] = "completed"
			taskUpdates["completed_at"] = now
		}
		if err := tx.Model(&model.VoteTask{}).Where("id = ?", taskID).Updates(taskUpdates).Error; err != nil {
			return fmt.Errorf("failed to update vote task: %w", err)
		}
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("failed to reload vote task: %w", err)
		}

		result.Task = task
		result.Record = record
		return nil
	})
	if err != nil {
		return nil, err
	}

	event := "save_vote_draft"
	if finalSubmit {
		event = "submit_vote"
	}
	targetID := result.Task.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "vote_tasks", &targetID, map[string]any{
		"event":       event,
		"gradeOption": result.Record.GradeOption,
	}, ipAddress, userAgent))
	return result, nil
}

func resolveVoteTaskObjectsTx(tx *gorm.DB, yearID uint, objectIDs []uint) ([]model.AssessmentObject, error) {
	query := tx.Model(&model.AssessmentObject{}).Where("year_id = ? AND is_active = 1", yearID)
	dedupIDs := make([]uint, 0, len(objectIDs))
	if len(objectIDs) > 0 {
		seen := make(map[uint]struct{}, len(objectIDs))
		for _, objectID := range objectIDs {
			if objectID == 0 {
				return nil, ErrInvalidParam
			}
			if _, exists := seen[objectID]; exists {
				continue
			}
			seen[objectID] = struct{}{}
			dedupIDs = append(dedupIDs, objectID)
		}
		query = query.Where("id IN ?", dedupIDs)
	}

	var objects []model.AssessmentObject
	if err := query.Order("id ASC").Find(&objects).Error; err != nil {
		return nil, fmt.Errorf("failed to query assessment objects: %w", err)
	}
	if len(dedupIDs) > 0 && len(objects) != len(dedupIDs) {
		return nil, ErrAssessmentObjectNotFound
	}
	return objects, nil
}

func resolveVoteGroupVotersTx(tx *gorm.DB, group model.VoteGroup) ([]uint, error) {
	scope := voteScope{}
	rawScope := strings.TrimSpace(group.VoterScope)
	if rawScope != "" {
		if err := json.Unmarshal([]byte(rawScope), &scope); err != nil {
			return nil, ErrInvalidParam
		}
	}

	query := tx.Model(&model.User{}).Where("status = ? AND deleted_at IS NULL", "active")
	hasScope := false
	if len(scope.UserIDs) > 0 {
		query = query.Where("users.id IN ?", scope.UserIDs)
		hasScope = true
	}
	if len(scope.OrganizationIDs) > 0 {
		query = query.Joins("JOIN user_organizations uo ON uo.user_id = users.id").Where("uo.organization_id IN ?", scope.OrganizationIDs)
		hasScope = true
	}
	if len(scope.RoleCodes) > 0 {
		normalizedRoleCodes := make([]string, 0, len(scope.RoleCodes))
		for _, roleCode := range scope.RoleCodes {
			if text := strings.TrimSpace(roleCode); text != "" {
				normalizedRoleCodes = append(normalizedRoleCodes, text)
			}
		}
		if len(normalizedRoleCodes) > 0 {
			query = query.
				Joins("JOIN user_roles ur ON ur.user_id = users.id").
				Joins("JOIN roles r ON r.id = ur.role_id").
				Where("r.role_code IN ?", normalizedRoleCodes)
			hasScope = true
		}
	}

	// custom 分组要求显式指定范围，避免误把所有用户都分配为投票人。
	if !hasScope && strings.EqualFold(strings.TrimSpace(group.VoterType), "custom") {
		return []uint{}, nil
	}

	var userIDs []uint
	if err := query.Distinct("users.id").Pluck("users.id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to resolve vote group voters: %w", err)
	}
	return userIDs, nil
}
