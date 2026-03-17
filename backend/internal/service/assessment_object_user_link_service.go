package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

const (
	defaultObjectLinkType = "member"
	objectLinkAccessRead  = "read"
)

type AssessmentObjectUserLinkService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type AssessmentObjectUserLinkItem struct {
	ID                       uint   `json:"id"`
	UserID                   uint   `json:"userId"`
	AssessmentObjectID       uint   `json:"assessmentObjectId"`
	AssessmentObjectName     string `json:"assessmentObjectName"`
	AssessmentObjectYear     uint   `json:"assessmentObjectYear"`
	AssessmentObjectType     string `json:"assessmentObjectType"`
	AssessmentObjectCategory string `json:"assessmentObjectCategory"`
	AssessmentObjectAlive    bool   `json:"assessmentObjectActive"`
	LinkType                 string `json:"linkType"`
	AccessLevel              string `json:"accessLevel"`
	IsPrimary                bool   `json:"isPrimary"`
	EffectiveFrom            *int64 `json:"effectiveFrom,omitempty"`
	EffectiveTo              *int64 `json:"effectiveTo,omitempty"`
	IsActive                 bool   `json:"isActive"`
	CreatedBy                *uint  `json:"createdBy,omitempty"`
	CreatedAt                int64  `json:"createdAt"`
	UpdatedBy                *uint  `json:"updatedBy,omitempty"`
	UpdatedAt                int64  `json:"updatedAt"`
}

type ReplaceAssessmentObjectUserLinksInput struct {
	Items []AssessmentObjectUserLinkUpsertItem `json:"items"`
}

type AssessmentObjectUserLinkUpsertItem struct {
	AssessmentObjectID uint   `json:"assessmentObjectId"`
	LinkType           string `json:"linkType"`
	AccessLevel        string `json:"accessLevel"`
	IsPrimary          bool   `json:"isPrimary"`
	EffectiveFrom      *int64 `json:"effectiveFrom,omitempty"`
	EffectiveTo        *int64 `json:"effectiveTo,omitempty"`
	IsActive           *bool  `json:"isActive,omitempty"`
}

func NewAssessmentObjectUserLinkService(db *gorm.DB, auditRepo *repository.AuditRepository) *AssessmentObjectUserLinkService {
	return &AssessmentObjectUserLinkService{
		db:        db,
		auditRepo: auditRepo,
	}
}

func (s *AssessmentObjectUserLinkService) ListUserLinks(
	ctx context.Context,
	userID uint,
	yearID *uint,
) ([]AssessmentObjectUserLinkItem, error) {
	if userID == 0 {
		return nil, ErrInvalidParam
	}

	query := s.db.WithContext(ctx).
		Table("assessment_object_user_links AS l").
		Select(`
l.id,
l.user_id,
l.assessment_object_id,
ao.object_name AS assessment_object_name,
ao.year_id AS assessment_object_year,
ao.object_type AS assessment_object_type,
ao.object_category AS assessment_object_category,
ao.is_active AS assessment_object_alive,
l.link_type,
l.access_level,
l.is_primary,
l.effective_from,
l.effective_to,
l.is_active,
l.created_by,
l.created_at,
l.updated_by,
l.updated_at`).
		Joins("JOIN assessment_objects AS ao ON ao.id = l.assessment_object_id").
		Where("l.user_id = ?", userID)
	if yearID != nil && *yearID > 0 {
		query = query.Where("ao.year_id = ?", *yearID)
	}

	items := make([]AssessmentObjectUserLinkItem, 0, 16)
	if err := query.
		Order("ao.year_id DESC").
		Order("l.assessment_object_id ASC").
		Order("l.link_type ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to query assessment object user links: %w", err)
	}
	return items, nil
}

func (s *AssessmentObjectUserLinkService) ReplaceUserLinks(
	ctx context.Context,
	operatorID uint,
	userID uint,
	input ReplaceAssessmentObjectUserLinksInput,
	ipAddress string,
	userAgent string,
) ([]AssessmentObjectUserLinkItem, error) {
	if userID == 0 {
		return nil, ErrInvalidParam
	}

	normalizedItems, objectIDs, err := normalizeObjectUserLinkUpsertItems(input.Items)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAssessmentObjectsExist(ctx, objectIDs); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		operatorRef := resolveBusinessWriteOperatorRefTx(tx, operatorID)

		if err := tx.Where("user_id = ?", userID).
			Delete(&model.AssessmentObjectUserLink{}).Error; err != nil {
			return fmt.Errorf("failed to clear assessment object links: %w", err)
		}
		if len(normalizedItems) == 0 {
			return nil
		}

		records := make([]model.AssessmentObjectUserLink, 0, len(normalizedItems))
		for _, item := range normalizedItems {
			isActive := true
			if item.IsActive != nil {
				isActive = *item.IsActive
			}
			records = append(records, model.AssessmentObjectUserLink{
				UserID:             userID,
				AssessmentObjectID: item.AssessmentObjectID,
				LinkType:           item.LinkType,
				AccessLevel:        item.AccessLevel,
				IsPrimary:          item.IsPrimary,
				EffectiveFrom:      item.EffectiveFrom,
				EffectiveTo:        item.EffectiveTo,
				IsActive:           isActive,
				CreatedBy:          operatorRef,
				CreatedAt:          now,
				UpdatedBy:          operatorRef,
				UpdatedAt:          now,
			})
		}

		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("failed to create assessment object links: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if s.auditRepo != nil {
		operatorRef := operatorID
		targetID := userID
		_ = s.auditRepo.Create(ctx, buildAuditRecord(
			&operatorRef,
			"update",
			"assessment_object_user_links",
			&targetID,
			map[string]any{
				"event":  "replace_user_object_links",
				"userId": userID,
				"count":  len(normalizedItems),
			},
			ipAddress,
			userAgent,
		))
	}

	return s.ListUserLinks(ctx, userID, nil)
}

func (s *AssessmentObjectUserLinkService) ensureAssessmentObjectsExist(ctx context.Context, objectIDs []uint) error {
	if len(objectIDs) == 0 {
		return nil
	}

	existing := make([]uint, 0, len(objectIDs))
	if err := s.db.WithContext(ctx).
		Table("assessment_objects").
		Where("id IN ?", objectIDs).
		Pluck("id", &existing).Error; err != nil {
		return fmt.Errorf("failed to verify assessment objects: %w", err)
	}

	existingSet := make(map[uint]struct{}, len(existing))
	for _, objectID := range existing {
		existingSet[objectID] = struct{}{}
	}
	for _, objectID := range objectIDs {
		if _, ok := existingSet[objectID]; ok {
			continue
		}
		return ErrAssessmentObjectNotFound
	}
	return nil
}

func normalizeObjectUserLinkUpsertItems(
	items []AssessmentObjectUserLinkUpsertItem,
) ([]AssessmentObjectUserLinkUpsertItem, []uint, error) {
	if len(items) == 0 {
		return []AssessmentObjectUserLinkUpsertItem{}, []uint{}, nil
	}

	normalized := make([]AssessmentObjectUserLinkUpsertItem, 0, len(items))
	objectIDs := make([]uint, 0, len(items))
	objectIDSet := make(map[uint]struct{}, len(items))
	uniqueKeys := make(map[string]struct{}, len(items))

	for _, item := range items {
		if item.AssessmentObjectID == 0 {
			return nil, nil, ErrInvalidParam
		}

		linkType := strings.ToLower(strings.TrimSpace(item.LinkType))
		if linkType == "" {
			linkType = defaultObjectLinkType
		}
		if len(linkType) > 30 {
			return nil, nil, ErrInvalidParam
		}

		accessLevel := strings.ToLower(strings.TrimSpace(item.AccessLevel))
		if accessLevel == "" {
			accessLevel = assessmentObjectAccessDetail
		}
		if accessLevel != objectLinkAccessRead && accessLevel != assessmentObjectAccessDetail {
			return nil, nil, ErrInvalidParam
		}

		if item.EffectiveFrom != nil && item.EffectiveTo != nil && *item.EffectiveTo < *item.EffectiveFrom {
			return nil, nil, ErrInvalidParam
		}

		uniqueKey := fmt.Sprintf("%d|%s", item.AssessmentObjectID, linkType)
		if _, exists := uniqueKeys[uniqueKey]; exists {
			return nil, nil, ErrInvalidParam
		}
		uniqueKeys[uniqueKey] = struct{}{}

		normalized = append(normalized, AssessmentObjectUserLinkUpsertItem{
			AssessmentObjectID: item.AssessmentObjectID,
			LinkType:           linkType,
			AccessLevel:        accessLevel,
			IsPrimary:          item.IsPrimary,
			EffectiveFrom:      item.EffectiveFrom,
			EffectiveTo:        item.EffectiveTo,
			IsActive:           item.IsActive,
		})

		if _, exists := objectIDSet[item.AssessmentObjectID]; exists {
			continue
		}
		objectIDSet[item.AssessmentObjectID] = struct{}{}
		objectIDs = append(objectIDs, item.AssessmentObjectID)
	}

	return normalized, objectIDs, nil
}
