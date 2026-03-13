package model

// Permission bits (similar to Linux rwx)
const (
	PermRead    uint8 = 1 << 3 // R: Read permission
	PermWrite   uint8 = 1 << 2 // W: Write permission
	PermDelete  uint8 = 1 << 1 // D: Delete permission
	PermExecute uint8 = 1 << 0 // X: Execute permission (special operations like approve, submit)
)

// Permission layers (similar to Linux owner/group/others)
const (
	LayerOwner  = "owner"
	LayerGroup  = "group"
	LayerOthers = "others"
)

// ResourcePermission represents permission configuration for a resource
type ResourcePermission struct {
	PermissionMode uint16 `json:"permissionMode"` // Combined permission mode, e.g., 0644
}

// ParsePermissionMode parses a permission mode (e.g., 0644) into layer permissions
// Returns (ownerPerms, groupPerms, othersPerms)
func ParsePermissionMode(mode uint16) (uint8, uint8, uint8) {
	owner := uint8((mode >> 6) & 0xF)
	group := uint8((mode >> 3) & 0xF)
	others := uint8(mode & 0xF)
	return owner, group, others
}

// MakePermissionMode creates a permission mode from layer permissions
func MakePermissionMode(owner, group, others uint8) uint16 {
	return uint16(owner)<<6 | uint16(group)<<3 | uint16(others)
}

// HasPermission checks if a permission layer has a specific action permission
func HasPermission(layerPerms uint8, action string) bool {
	var requiredPerm uint8
	switch action {
	case "read":
		requiredPerm = PermRead
	case "write":
		requiredPerm = PermWrite
	case "delete":
		requiredPerm = PermDelete
	case "execute":
		requiredPerm = PermExecute
	default:
		return false
	}
	return (layerPerms & requiredPerm) != 0
}
