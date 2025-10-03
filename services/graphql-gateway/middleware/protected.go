package middleware

import (
	"context"
	"fmt"
)

// ProtectedOperation wraps GraphQL operations that require authentication
type ProtectedOperation[T any] func(ctx context.Context, user *CurrentUser) (T, error)

// WithAuth wraps a resolver function to require authentication
func WithAuth[T any](ctx context.Context, operation ProtectedOperation[T]) (T, error) {
	var zero T

	user, err := RequireAuth(ctx)
	if err != nil {
		return zero, fmt.Errorf("authentication required: %w", err)
	}

	return operation(ctx, user)
}

// WithOptionalAuth wraps a resolver function with optional authentication
func WithOptionalAuth[T any](ctx context.Context, operation func(ctx context.Context, user *CurrentUser) (T, error)) (T, error) {
	user := GetCurrentUser(ctx)
	return operation(ctx, user)
}

// AuthenticatedResolver is a helper interface for resolvers that need authentication
type AuthenticatedResolver interface {
	RequireAuth(ctx context.Context) (*CurrentUser, error)
	GetCurrentUser(ctx context.Context) *CurrentUser
}

// BaseAuthenticatedResolver provides common auth functionality
type BaseAuthenticatedResolver struct{}

func (r *BaseAuthenticatedResolver) RequireAuth(ctx context.Context) (*CurrentUser, error) {
	return RequireAuth(ctx)
}

func (r *BaseAuthenticatedResolver) GetCurrentUser(ctx context.Context) *CurrentUser {
	return GetCurrentUser(ctx)
}
