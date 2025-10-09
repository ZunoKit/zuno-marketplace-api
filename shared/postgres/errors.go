package postgres

import (
	"strings"

	"github.com/lib/pq"
)

// IsUniqueViolation checks if the error is a unique constraint violation
func IsUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		return false
	}

	// 23505 is the PostgreSQL error code for unique_violation
	if pqErr.Code != "23505" {
		return false
	}

	// If constraintName is provided, check if it matches
	if constraintName != "" {
		return strings.Contains(pqErr.Detail, constraintName) ||
			strings.Contains(pqErr.Constraint, constraintName)
	}

	return true
}

// IsForeignKeyViolation checks if the error is a foreign key constraint violation
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		return false
	}

	// 23503 is the PostgreSQL error code for foreign_key_violation
	return pqErr.Code == "23503"
}

// IsNotNullViolation checks if the error is a not null constraint violation
func IsNotNullViolation(err error) bool {
	if err == nil {
		return false
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		return false
	}

	// 23502 is the PostgreSQL error code for not_null_violation
	return pqErr.Code == "23502"
}

// IsCheckViolation checks if the error is a check constraint violation
func IsCheckViolation(err error) bool {
	if err == nil {
		return false
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		return false
	}

	// 23514 is the PostgreSQL error code for check_violation
	return pqErr.Code == "23514"
}
