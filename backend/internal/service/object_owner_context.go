package service

import (
	"fmt"
	"strings"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

type objectOwnerContext struct {
	BelongOrgID  *uint
	OwnerOrgID   *uint
	OwnerOrgType string
}

func buildObjectOwnerContextMapTx(
	tx *gorm.DB,
	objects []model.AssessmentObject,
	segmentMap map[uint]string,
) (map[uint]objectOwnerContext, error) {
	result := make(map[uint]objectOwnerContext, len(objects))
	if len(objects) == 0 {
		return result, nil
	}

	departmentTargetIDs := make([]uint, 0)
	employeeTargetIDs := make([]uint, 0)
	for _, object := range objects {
		switch strings.ToLower(strings.TrimSpace(object.TargetType)) {
		case "department":
			departmentTargetIDs = append(departmentTargetIDs, object.TargetID)
		case "employee":
			employeeTargetIDs = append(employeeTargetIDs, object.TargetID)
		}
	}

	departmentOrgMap := map[uint]uint{}
	if len(departmentTargetIDs) > 0 {
		type departmentOwnerRow struct {
			ID             uint
			OrganizationID uint
		}
		var rows []departmentOwnerRow
		if err := tx.Table("departments").
			Select("id, organization_id").
			Where("id IN ?", departmentTargetIDs).
			Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("failed to load department owner context: %w", err)
		}
		for _, row := range rows {
			departmentOrgMap[row.ID] = row.OrganizationID
		}
	}

	employeeOrgMap := map[uint]uint{}
	if len(employeeTargetIDs) > 0 {
		type employeeOwnerRow struct {
			ID             uint
			OrganizationID uint
		}
		var rows []employeeOwnerRow
		if err := tx.Table("employees").
			Select("id, organization_id").
			Where("id IN ?", employeeTargetIDs).
			Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("failed to load employee owner context: %w", err)
		}
		for _, row := range rows {
			employeeOrgMap[row.ID] = row.OrganizationID
		}
	}

	belongOrgByObject := make(map[uint]*uint, len(objects))
	orgIDs := map[uint]struct{}{}
	for _, object := range objects {
		var belongOrgID *uint
		switch strings.ToLower(strings.TrimSpace(object.TargetType)) {
		case "organization", "leadership_team":
			if object.TargetID > 0 {
				belongOrgID = uintPtr(object.TargetID)
			}
		case "department":
			if organizationID, ok := departmentOrgMap[object.TargetID]; ok && organizationID > 0 {
				belongOrgID = uintPtr(organizationID)
			}
		case "employee":
			if organizationID, ok := employeeOrgMap[object.TargetID]; ok && organizationID > 0 {
				belongOrgID = uintPtr(organizationID)
			}
		}
		belongOrgByObject[object.ID] = belongOrgID
		if belongOrgID != nil {
			orgIDs[*belongOrgID] = struct{}{}
		}
	}

	type organizationRow struct {
		ID       uint
		OrgType  string
		ParentID *uint
	}
	orgRowByID := map[uint]organizationRow{}
	if len(orgIDs) > 0 {
		organizationIDs := make([]uint, 0, len(orgIDs))
		for organizationID := range orgIDs {
			organizationIDs = append(organizationIDs, organizationID)
		}

		var rows []organizationRow
		if err := tx.Table("organizations").
			Select("id, org_type, parent_id").
			Where("id IN ?", organizationIDs).
			Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("failed to load organization owner context: %w", err)
		}
		parentIDs := map[uint]struct{}{}
		for _, row := range rows {
			orgRowByID[row.ID] = row
			if row.ParentID != nil && *row.ParentID > 0 {
				parentIDs[*row.ParentID] = struct{}{}
			}
		}
		if len(parentIDs) > 0 {
			missingParentIDs := make([]uint, 0, len(parentIDs))
			for parentID := range parentIDs {
				if _, exists := orgRowByID[parentID]; exists {
					continue
				}
				missingParentIDs = append(missingParentIDs, parentID)
			}
			if len(missingParentIDs) > 0 {
				var parentRows []organizationRow
				if err := tx.Table("organizations").
					Select("id, org_type, parent_id").
					Where("id IN ?", missingParentIDs).
					Find(&parentRows).Error; err != nil {
					return nil, fmt.Errorf("failed to load parent organization owner context: %w", err)
				}
				for _, row := range parentRows {
					orgRowByID[row.ID] = row
				}
			}
		}
	}

	for _, object := range objects {
		segmentCode := segmentMap[object.ID]
		belongOrgID := belongOrgByObject[object.ID]
		ownerOrgID := belongOrgID
		if isChildRelationSegmentCode(segmentCode) && belongOrgID != nil {
			if orgRow, ok := orgRowByID[*belongOrgID]; ok && orgRow.ParentID != nil && *orgRow.ParentID > 0 {
				ownerOrgID = uintPtr(*orgRow.ParentID)
			}
		}

		ownerOrgType := ""
		if ownerOrgID != nil {
			if row, ok := orgRowByID[*ownerOrgID]; ok {
				ownerOrgType = strings.ToLower(strings.TrimSpace(row.OrgType))
			}
		}
		result[object.ID] = objectOwnerContext{
			BelongOrgID:  belongOrgID,
			OwnerOrgID:   ownerOrgID,
			OwnerOrgType: ownerOrgType,
		}
	}
	return result, nil
}

func isChildRelationSegmentCode(segmentCode string) bool {
	normalized := normalizeSegmentCode(segmentCode)
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, "SEG_CHILD_")
}
