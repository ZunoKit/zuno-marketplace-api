package domain

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidUserID   = errors.New("invalid user ID")
	ErrInvalidAddress  = errors.New("invalid address")
	ErrUserBanned      = errors.New("user is banned")
	ErrUserDeleted     = errors.New("user is deleted")
)
