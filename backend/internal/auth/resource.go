package auth

import (
	"assessv2/backend/internal/model"
)

// ResourceRelation represents the relationship between a user and a resource
type ResourceRelation string

const (
	RelationOwner  ResourceRelation = "owner"
	RelationGroup  ResourceRelation = "group"
	RelationOthers ResourceRelation = "others"
)

// ResourceWithPermission interface for resources that have permission control
type ResourceWithPermission interface {
	GetOwnerID() uint
	GetPermissionMode() uint16
}

// GetUserResourceRelation determines the relationship between a user and a resource
// Returns owner if user created the resource, group if same organization, otherwise others
func GetUserResourceRelation(userID uint, ownerID uint, userOrgScopes []OrganizationScope, resourceOrgType string, resourceOrgID uint) ResourceRelation {
	// Check if user is the owner
	if userID == ownerID {
		return RelationOwner
	}

	// Check if user is in the same organization (group)
	for _, scope := range userOrgScopes {
		if scope.OrganizationType == resourceOrgType && scope.OrganizationID == resourceOrgID {
			return RelationGroup
		}
	}

	return RelationOthers
}

// CheckResourcePermission checks if a user has permission to perform an action on a resource
// root role always has full access
func CheckResourcePermission(
	userID uint,
	roles []string,
	ownerID uint,
	permissionMode uint16,
	userOrgScopes []OrganizationScope,
	resourceOrgType string,
	resourceOrgID uint,
	action string,
) bool {
	// root role bypasses all resource-level checks
	if HasRole(roles, "root") {
		return true
	}

	// Determine user's relation to the resource
	relation := GetUserResourceRelation(userID, ownerID, userOrgScopes, resourceOrgType, resourceOrgID)

	// Parse permission mode
	ownerPerms, groupPerms, othersPerms := model.ParsePermissionMode(permissionMode)

	// Check permission based on relation
	var layerPerms uint8
	switch relation {
	case RelationOwner:
		layerPerms = ownerPerms
	case RelationGroup:
		layerPerms = groupPerms
	case RelationOthers:
		layerPerms = othersPerms
	default:
		return false
	}

	return model.HasPermission(layerPerms, action)
}
