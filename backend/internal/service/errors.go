package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrAccountLocked      = errors.New("account is locked")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrInvalidUserStatus  = errors.New("invalid user status")
	ErrCannotDisableSelf  = errors.New("cannot disable current user")
	ErrInvalidRoleCode    = errors.New("invalid role code")
	ErrInvalidRoleName    = errors.New("invalid role name")
	ErrRoleCodeExists     = errors.New("role code already exists")
	ErrRoleNotFound       = errors.New("role not found")
	ErrSystemRoleLocked   = errors.New("system role is immutable")
	ErrRoleInUse          = errors.New("role is still assigned to users")
	ErrInvalidRoleList    = errors.New("invalid role list")
	ErrCannotDemoteRoot   = errors.New("root user must keep root role")
)
