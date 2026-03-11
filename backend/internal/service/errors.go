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
)
