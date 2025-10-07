package domain

import (
	"errors"
	"fmt"
)

// Domain errors
var (
	ErrUserNotFound      = errors.New("user_not_found")
	ErrProfileNotFound   = errors.New("profile_not_found")
	ErrDuplicateUser     = errors.New("duplicate_user")
	ErrInvalidInput      = errors.New("invalid_input")
	ErrInvalidAccountID  = errors.New("invalid_account_id")
	ErrInvalidAddress    = errors.New("invalid_address")
	ErrInvalidChainID    = errors.New("invalid_chain_id")
	ErrDatabaseOperation = errors.New("database_operation_failed")
	ErrAccountExists     = errors.New("account_already_exists")
)

// Error helpers
func NewInvalidInputError(field string, reason string) error {
	return fmt.Errorf("%w: %s - %s", ErrInvalidInput, field, reason)
}

func NewDatabaseError(operation string, err error) error {
	return fmt.Errorf("%w: %s - %v", ErrDatabaseOperation, operation, err)
}

// User status constants
const (
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
	UserStatusDeleted   = "deleted"
)

// Default profile values
const (
	DefaultLocale   = "en"
	DefaultTimezone = "UTC"
)
