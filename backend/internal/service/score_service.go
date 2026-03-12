package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

type ScoreService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type ListDirectScoreFilter struct {
	YearID     *uint
	PeriodCode string
	ModuleID   *uint
	ObjectID   *uint
}

type CreateDirectScoreInput struct {
	YearID     uint
	PeriodCode string
	ModuleID   uint
	ObjectID   uint
	Score      float64
	Remark     string
}

type UpdateDirectScoreInput struct {
	Score  float64
	Remark string
}

type BatchDirectScoreEntry struct {
	ObjectID uint
	Score    float64
	Remark   string
}

type BatchDirectScoreInput struct {
	YearID     uint
	PeriodCode string
	ModuleID   uint
	Overwrite  bool
	Entries    []BatchDirectScoreEntry
}

type BatchDirectScoreResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

type ListExtraPointFilter struct {
	YearID     *uint
	PeriodCode string
	ObjectID   *uint
	PointType  string
}

type CreateExtraPointInput struct {
	YearID     uint
	PeriodCode string
	ObjectID   uint
	PointType  string
	Points     float64
	Reason     string
	Evidence   string
	Approve    bool
}

type UpdateExtraPointInput struct {
	PointType string
	Points    float64
	Reason    string
	Evidence  string
	Approve   *bool
}

func NewScoreService(db *gorm.DB, auditRepo *repository.AuditRepository) *ScoreService {
	return &ScoreService{db: db, auditRepo: auditRepo}
}

func (s *ScoreService) ListDirectScores(ctx context.Context, filter ListDirectScoreFilter) ([]model.DirectScore, error) {
	query := s.db.WithContext(ctx).Model(&model.DirectScore{})
	if filter.YearID != nil {
		query = query.Where("year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("period_code = ?", periodCode)
	}
	if filter.ModuleID != nil {
		query = query.Where("module_id = ?", *filter.ModuleID)
	}
	if filter.ObjectID != nil {
		query = query.Where("object_id = ?", *filter.ObjectID)
	}

	var items []model.DirectScore
	if err := query.Order("id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list direct scores: %w", err)
	}
	return items, nil
}

func (s *ScoreService) CreateDirectScore(
	ctx context.Context,
	operatorID uint,
	input CreateDirectScoreInput,
	ipAddress string,
	userAgent string,
) (*model.DirectScore, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ModuleID == 0 || input.ObjectID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}

	operator := operatorID
	now := time.Now().Unix()
	record := &model.DirectScore{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePeriodWritableTx(tx, input.YearID, periodCode); err != nil {
			return err
		}
		module, err := loadModuleByPeriodTx(tx, input.ModuleID, "direct", input.YearID, periodCode)
		if err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrInvalidScoreModule
			}
			return fmt.Errorf("failed to query direct score module: %w", err)
		}
		if module.MaxScore == nil || *module.MaxScore <= 0 {
			return ErrInvalidScoreModule
		}
		if input.Score < 0 || input.Score > *module.MaxScore {
			return ErrInvalidScoreValue
		}
		if _, err := ensureAssessmentObjectTx(tx, input.ObjectID, input.YearID); err != nil {
			return err
		}

		var existing model.DirectScore
		if err := tx.Where(
			"year_id = ? AND period_code = ? AND module_id = ? AND object_id = ?",
			input.YearID, periodCode, input.ModuleID, input.ObjectID,
		).First(&existing).Error; err == nil {
			return ErrDirectScoreExists
		} else if !repository.IsRecordNotFound(err) {
			return fmt.Errorf("failed to verify direct score duplicate: %w", err)
		}

		*record = model.DirectScore{
			YearID:     input.YearID,
			PeriodCode: periodCode,
			ModuleID:   input.ModuleID,
			ObjectID:   input.ObjectID,
			Score:      roundToScale(input.Score, 6),
			Remark:     strings.TrimSpace(input.Remark),
			InputBy:    operator,
			InputAt:    now,
		}
		if err := tx.Create(record).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrDirectScoreExists
			}
			return fmt.Errorf("failed to create direct score: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "direct_scores", &targetID, map[string]any{
		"event":      "create_direct_score",
		"yearId":     record.YearID,
		"periodCode": record.PeriodCode,
		"moduleId":   record.ModuleID,
		"objectId":   record.ObjectID,
		"score":      record.Score,
	}, ipAddress, userAgent))
	return record, nil
}

func (s *ScoreService) BatchUpsertDirectScores(
	ctx context.Context,
	operatorID uint,
	input BatchDirectScoreInput,
	ipAddress string,
	userAgent string,
) (*BatchDirectScoreResult, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ModuleID == 0 || !isValidPeriodCode(periodCode) || len(input.Entries) == 0 {
		return nil, ErrInvalidParam
	}

	entryMap := make(map[uint]BatchDirectScoreEntry, len(input.Entries))
	objectIDs := make([]uint, 0, len(input.Entries))
	for _, entry := range input.Entries {
		if entry.ObjectID == 0 {
			return nil, ErrInvalidParam
		}
		if _, exists := entryMap[entry.ObjectID]; exists {
			return nil, ErrInvalidParam
		}
		entryMap[entry.ObjectID] = BatchDirectScoreEntry{
			ObjectID: entry.ObjectID,
			Score:    roundToScale(entry.Score, 6),
			Remark:   strings.TrimSpace(entry.Remark),
		}
		objectIDs = append(objectIDs, entry.ObjectID)
	}

	operator := operatorID
	now := time.Now().Unix()
	result := &BatchDirectScoreResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePeriodWritableTx(tx, input.YearID, periodCode); err != nil {
			return err
		}
		module, err := loadModuleByPeriodTx(tx, input.ModuleID, "direct", input.YearID, periodCode)
		if err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrInvalidScoreModule
			}
			return fmt.Errorf("failed to query direct score module: %w", err)
		}
		if module.MaxScore == nil || *module.MaxScore <= 0 {
			return ErrInvalidScoreModule
		}

		var objectCount int64
		if err := tx.Model(&model.AssessmentObject{}).
			Where("year_id = ? AND is_active = 1 AND id IN ?", input.YearID, objectIDs).
			Count(&objectCount).Error; err != nil {
			return fmt.Errorf("failed to verify assessment objects: %w", err)
		}
		if int(objectCount) != len(objectIDs) {
			return ErrAssessmentObjectNotFound
		}

		var existing []model.DirectScore
		if err := tx.Where(
			"year_id = ? AND period_code = ? AND module_id = ? AND object_id IN ?",
			input.YearID, periodCode, input.ModuleID, objectIDs,
		).Find(&existing).Error; err != nil {
			return fmt.Errorf("failed to query existing direct scores: %w", err)
		}
		existingMap := make(map[uint]model.DirectScore, len(existing))
		for _, item := range existing {
			existingMap[item.ObjectID] = item
		}

		for _, objectID := range objectIDs {
			entry := entryMap[objectID]
			if entry.Score < 0 || entry.Score > *module.MaxScore {
				return ErrInvalidScoreValue
			}
			existingRecord, exists := existingMap[objectID]
			if !exists {
				record := model.DirectScore{
					YearID:     input.YearID,
					PeriodCode: periodCode,
					ModuleID:   input.ModuleID,
					ObjectID:   objectID,
					Score:      entry.Score,
					Remark:     entry.Remark,
					InputBy:    operator,
					InputAt:    now,
				}
				if err := tx.Create(&record).Error; err != nil {
					return fmt.Errorf("failed to create direct score in batch: %w", err)
				}
				result.Created++
				continue
			}

			if !input.Overwrite {
				result.Skipped++
				continue
			}
			if err := tx.Model(&model.DirectScore{}).
				Where("id = ?", existingRecord.ID).
				Updates(map[string]any{
					"score":      entry.Score,
					"remark":     entry.Remark,
					"updated_by": &operator,
					"updated_at": now,
				}).Error; err != nil {
				return fmt.Errorf("failed to update direct score in batch: %w", err)
			}
			result.Updated++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "direct_scores", nil, map[string]any{
		"event":      "batch_upsert_direct_scores",
		"yearId":     input.YearID,
		"periodCode": periodCode,
		"moduleId":   input.ModuleID,
		"overwrite":  input.Overwrite,
		"created":    result.Created,
		"updated":    result.Updated,
		"skipped":    result.Skipped,
	}, ipAddress, userAgent))
	return result, nil
}

func (s *ScoreService) UpdateDirectScore(
	ctx context.Context,
	operatorID uint,
	scoreID uint,
	input UpdateDirectScoreInput,
	ipAddress string,
	userAgent string,
) (*model.DirectScore, error) {
	if scoreID == 0 {
		return nil, ErrInvalidParam
	}

	operator := operatorID
	now := time.Now().Unix()
	var record model.DirectScore
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", scoreID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrDirectScoreNotFound
			}
			return fmt.Errorf("failed to query direct score: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, record.YearID, record.PeriodCode); err != nil {
			return err
		}
		module, err := loadModuleByPeriodTx(tx, record.ModuleID, "direct", record.YearID, record.PeriodCode)
		if err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrInvalidScoreModule
			}
			return fmt.Errorf("failed to query direct score module: %w", err)
		}
		if module.MaxScore == nil || *module.MaxScore <= 0 {
			return ErrInvalidScoreModule
		}
		scoreValue := roundToScale(input.Score, 6)
		if scoreValue < 0 || scoreValue > *module.MaxScore {
			return ErrInvalidScoreValue
		}
		if err := tx.Model(&model.DirectScore{}).Where("id = ?", scoreID).Updates(map[string]any{
			"score":      scoreValue,
			"remark":     strings.TrimSpace(input.Remark),
			"updated_by": &operator,
			"updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("failed to update direct score: %w", err)
		}
		if err := tx.Where("id = ?", scoreID).First(&record).Error; err != nil {
			return fmt.Errorf("failed to reload direct score: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "direct_scores", &targetID, map[string]any{
		"event": "update_direct_score",
		"score": record.Score,
	}, ipAddress, userAgent))
	return &record, nil
}

func (s *ScoreService) DeleteDirectScore(
	ctx context.Context,
	operatorID uint,
	scoreID uint,
	ipAddress string,
	userAgent string,
) error {
	if scoreID == 0 {
		return ErrInvalidParam
	}

	operator := operatorID
	var record model.DirectScore
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", scoreID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrDirectScoreNotFound
			}
			return fmt.Errorf("failed to query direct score: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, record.YearID, record.PeriodCode); err != nil {
			return err
		}
		if err := tx.Delete(&model.DirectScore{}, scoreID).Error; err != nil {
			return fmt.Errorf("failed to delete direct score: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "delete", "direct_scores", &targetID, map[string]any{
		"event":      "delete_direct_score",
		"yearId":     record.YearID,
		"periodCode": record.PeriodCode,
		"moduleId":   record.ModuleID,
		"objectId":   record.ObjectID,
	}, ipAddress, userAgent))
	return nil
}

func (s *ScoreService) ListExtraPoints(ctx context.Context, filter ListExtraPointFilter) ([]model.ExtraPoint, error) {
	query := s.db.WithContext(ctx).Model(&model.ExtraPoint{})
	if filter.YearID != nil {
		query = query.Where("year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("period_code = ?", periodCode)
	}
	if filter.ObjectID != nil {
		query = query.Where("object_id = ?", *filter.ObjectID)
	}
	if pointType := strings.ToLower(strings.TrimSpace(filter.PointType)); pointType != "" {
		query = query.Where("point_type = ?", pointType)
	}

	var items []model.ExtraPoint
	if err := query.Order("id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list extra points: %w", err)
	}
	return items, nil
}

func (s *ScoreService) CreateExtraPoint(
	ctx context.Context,
	operatorID uint,
	input CreateExtraPointInput,
	ipAddress string,
	userAgent string,
) (*model.ExtraPoint, error) {
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || input.ObjectID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, ErrExtraPointReasonEmpty
	}
	pointType, points, err := normalizeExtraPointInput(input.PointType, input.Points)
	if err != nil {
		return nil, err
	}

	operator := operatorID
	now := time.Now().Unix()
	record := &model.ExtraPoint{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePeriodWritableTx(tx, input.YearID, periodCode); err != nil {
			return err
		}
		if _, err := ensureAssessmentObjectTx(tx, input.ObjectID, input.YearID); err != nil {
			return err
		}

		*record = model.ExtraPoint{
			YearID:     input.YearID,
			PeriodCode: periodCode,
			ObjectID:   input.ObjectID,
			PointType:  pointType,
			Points:     points,
			Reason:     reason,
			Evidence:   strings.TrimSpace(input.Evidence),
			InputBy:    operator,
			InputAt:    now,
		}
		if input.Approve {
			record.ApprovedBy = &operator
			record.ApprovedAt = &now
		}
		if err := tx.Create(record).Error; err != nil {
			return fmt.Errorf("failed to create extra point: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "extra_points", &targetID, map[string]any{
		"event":        "create_extra_point",
		"yearId":       record.YearID,
		"periodCode":   record.PeriodCode,
		"objectId":     record.ObjectID,
		"pointType":    record.PointType,
		"points":       record.Points,
		"signedPoints": signedExtraPoints(record.PointType, record.Points),
		"approved":     record.ApprovedBy != nil,
	}, ipAddress, userAgent))
	return record, nil
}

func (s *ScoreService) UpdateExtraPoint(
	ctx context.Context,
	operatorID uint,
	extraPointID uint,
	input UpdateExtraPointInput,
	ipAddress string,
	userAgent string,
) (*model.ExtraPoint, error) {
	if extraPointID == 0 {
		return nil, ErrInvalidParam
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, ErrExtraPointReasonEmpty
	}
	pointType, points, err := normalizeExtraPointInput(input.PointType, input.Points)
	if err != nil {
		return nil, err
	}

	operator := operatorID
	now := time.Now().Unix()
	var record model.ExtraPoint
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", extraPointID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrExtraPointNotFound
			}
			return fmt.Errorf("failed to query extra point: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, record.YearID, record.PeriodCode); err != nil {
			return err
		}

		updates := map[string]any{
			"point_type": pointType,
			"points":     points,
			"reason":     reason,
			"evidence":   strings.TrimSpace(input.Evidence),
			"updated_by": &operator,
			"updated_at": now,
		}
		if input.Approve != nil {
			if *input.Approve {
				updates["approved_by"] = &operator
				updates["approved_at"] = now
			} else {
				updates["approved_by"] = nil
				updates["approved_at"] = nil
			}
		}
		if err := tx.Model(&model.ExtraPoint{}).Where("id = ?", extraPointID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update extra point: %w", err)
		}
		if err := tx.Where("id = ?", extraPointID).First(&record).Error; err != nil {
			return fmt.Errorf("failed to reload extra point: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "extra_points", &targetID, map[string]any{
		"event":        "update_extra_point",
		"pointType":    record.PointType,
		"points":       record.Points,
		"signedPoints": signedExtraPoints(record.PointType, record.Points),
		"approved":     record.ApprovedBy != nil,
	}, ipAddress, userAgent))
	return &record, nil
}

func (s *ScoreService) ApproveExtraPoint(
	ctx context.Context,
	operatorID uint,
	extraPointID uint,
	ipAddress string,
	userAgent string,
) (*model.ExtraPoint, error) {
	if extraPointID == 0 {
		return nil, ErrInvalidParam
	}

	operator := operatorID
	now := time.Now().Unix()
	var record model.ExtraPoint
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", extraPointID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrExtraPointNotFound
			}
			return fmt.Errorf("failed to query extra point: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, record.YearID, record.PeriodCode); err != nil {
			return err
		}
		if err := tx.Model(&model.ExtraPoint{}).Where("id = ?", extraPointID).Updates(map[string]any{
			"approved_by": &operator,
			"approved_at": now,
			"updated_by":  &operator,
			"updated_at":  now,
		}).Error; err != nil {
			return fmt.Errorf("failed to approve extra point: %w", err)
		}
		if err := tx.Where("id = ?", extraPointID).First(&record).Error; err != nil {
			return fmt.Errorf("failed to reload extra point: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "extra_points", &targetID, map[string]any{
		"event": "approve_extra_point",
	}, ipAddress, userAgent))
	return &record, nil
}

func (s *ScoreService) DeleteExtraPoint(
	ctx context.Context,
	operatorID uint,
	extraPointID uint,
	ipAddress string,
	userAgent string,
) error {
	if extraPointID == 0 {
		return ErrInvalidParam
	}

	operator := operatorID
	var record model.ExtraPoint
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", extraPointID).First(&record).Error; err != nil {
			if repository.IsRecordNotFound(err) {
				return ErrExtraPointNotFound
			}
			return fmt.Errorf("failed to query extra point: %w", err)
		}
		if err := ensurePeriodWritableTx(tx, record.YearID, record.PeriodCode); err != nil {
			return err
		}
		if err := tx.Delete(&model.ExtraPoint{}, extraPointID).Error; err != nil {
			return fmt.Errorf("failed to delete extra point: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "delete", "extra_points", &targetID, map[string]any{
		"event": "delete_extra_point",
	}, ipAddress, userAgent))
	return nil
}

func normalizeExtraPointInput(pointType string, points float64) (string, float64, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(pointType))
	switch normalizedType {
	case "":
		if points == 0 {
			return "", 0, ErrInvalidExtraPointValue
		}
		if points > 0 {
			normalizedType = "add"
		} else {
			normalizedType = "deduct"
		}
	case "add", "deduct":
	default:
		return "", 0, ErrInvalidExtraPointType
	}

	normalizedPoints := roundToScale(math.Abs(points), 6)
	if normalizedPoints <= 0 || normalizedPoints > 20 {
		return "", 0, ErrInvalidExtraPointValue
	}
	return normalizedType, normalizedPoints, nil
}

func signedExtraPoints(pointType string, points float64) float64 {
	if strings.ToLower(strings.TrimSpace(pointType)) == "deduct" {
		return -math.Abs(points)
	}
	return math.Abs(points)
}
