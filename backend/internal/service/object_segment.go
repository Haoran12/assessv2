package service

import (
	"fmt"

	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

func resolveObjectSegmentCode(object model.AssessmentObject, parent *model.AssessmentObject) string {
	if normalized := normalizeSegmentCode(object.ObjectCategory); normalized != "" {
		return normalized
	}

	switch object.ObjectType {
	case ObjectTypeTeam:
		switch object.ObjectCategory {
		case TeamCategoryGroupDepartment, TeamCategorySubsidiaryCompanyDepartment:
			return SegmentSelfDeptTeam
		case TeamCategoryGroupLeadership, TeamCategorySubsidiaryCompanyLeadership:
			return SegmentChildLeadershipTeam
		default:
			return ""
		}
	case ObjectTypeIndividual:
		switch object.ObjectCategory {
		case IndividualCategoryDepartmentMain:
			return SegmentSelfDeptPersonMain
		case IndividualCategoryDepartmentDeputy:
			return SegmentSelfDeptPersonDeputy
		case IndividualCategoryGeneralManagementPersonnel:
			if parent != nil {
				switch parent.ObjectCategory {
				case TeamCategoryGroupLeadership, TeamCategorySubsidiaryCompanyLeadership:
					return SegmentChildLeadershipPersonMember
				}
			}
			return SegmentSelfDeptPersonMember
		case IndividualCategoryLeadershipMain:
			return SegmentChildLeadershipPersonMain
		case IndividualCategoryLeadershipDeputy:
			return SegmentChildLeadershipPersonDeputy
		default:
			return ""
		}
	default:
		return ""
	}
}

func buildObjectSegmentMapTx(tx *gorm.DB, objects []model.AssessmentObject) (map[uint]string, error) {
	segmentMap := make(map[uint]string, len(objects))
	if len(objects) == 0 {
		return segmentMap, nil
	}

	objectByID := make(map[uint]model.AssessmentObject, len(objects))
	missingParentSet := map[uint]struct{}{}
	for _, object := range objects {
		objectByID[object.ID] = object
		if object.ParentObjectID == nil {
			continue
		}
		if _, ok := objectByID[*object.ParentObjectID]; ok {
			continue
		}
		missingParentSet[*object.ParentObjectID] = struct{}{}
	}

	if len(missingParentSet) > 0 {
		missingParentIDs := make([]uint, 0, len(missingParentSet))
		for parentID := range missingParentSet {
			missingParentIDs = append(missingParentIDs, parentID)
		}
		var parentObjects []model.AssessmentObject
		if err := tx.Where("id IN ?", missingParentIDs).Find(&parentObjects).Error; err != nil {
			return nil, fmt.Errorf("failed to load parent assessment objects for segment mapping: %w", err)
		}
		for _, parent := range parentObjects {
			objectByID[parent.ID] = parent
		}
	}

	for _, object := range objects {
		var parent *model.AssessmentObject
		if object.ParentObjectID != nil {
			if parentObject, ok := objectByID[*object.ParentObjectID]; ok {
				copyParent := parentObject
				parent = &copyParent
			}
		}
		segmentMap[object.ID] = resolveObjectSegmentCode(object, parent)
	}
	return segmentMap, nil
}
