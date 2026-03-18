package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func resolveOrganizationIDs(ctx context.Context, db *gorm.DB, organizationID uint, includeDescendants bool) ([]uint, error) {
	if organizationID == 0 {
		return []uint{}, nil
	}

	var ids []uint
	if includeDescendants {
		err := db.WithContext(ctx).Raw(`
WITH RECURSIVE org_tree(id) AS (
    SELECT id
    FROM organizations
    WHERE id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT o.id
    FROM organizations o
    JOIN org_tree ot ON o.parent_id = ot.id
    WHERE o.deleted_at IS NULL
)
SELECT id FROM org_tree
`, organizationID).Scan(&ids).Error
		if err != nil {
			return nil, fmt.Errorf("failed to resolve descendant organizations: %w", err)
		}
		return ids, nil
	}

	err := db.WithContext(ctx).Table("organizations").
		Where("deleted_at IS NULL AND (id = ? OR parent_id = ?)", organizationID, organizationID).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("failed to resolve direct-scope organizations: %w", err)
	}
	return ids, nil
}
