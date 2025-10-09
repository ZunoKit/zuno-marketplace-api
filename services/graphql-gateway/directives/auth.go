package directives

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
)

// Auth directive for authentication
type Auth struct{}

// NewAuth creates a new auth directive
func NewAuth() *Auth {
	return &Auth{}
}

// Directive implements the @auth directive
func (a *Auth) Directive(ctx context.Context, obj interface{}, next graphql.Resolver, role *string) (interface{}, error) {
	// Check if user is authenticated
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, fmt.Errorf("unauthorized: authentication required")
	}

	// Check role if specified
	if role != nil && !a.hasRole(user, *role) {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	return next(ctx)
}

// hasRole checks if user has the required role
func (a *Auth) hasRole(user *User, requiredRole string) bool {
	for _, userRole := range user.Roles {
		if userRole == requiredRole {
			return true
		}
	}
	return false
}

// User represents the authenticated user
type User struct {
	ID      string
	Email   string
	Roles   []string
	Wallets []string
}

// contextKey for storing user in context
type contextKey string

const userContextKey contextKey = "user"

// WithUser adds user to context
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// GetUserFromContext retrieves user from context
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}
