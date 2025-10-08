package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// SentryMiddleware creates HTTP middleware for Sentry integration
func SentryMiddleware() func(http.Handler) http.Handler {
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic:         true,
		WaitForDelivery: false,
		Timeout:         3 * time.Second,
	})

	return func(next http.Handler) http.Handler {
		return sentryHandler.Handle(next)
	}
}

// SentryGraphQLMiddleware creates GraphQL middleware for Sentry error tracking
func SentryGraphQLMiddleware() graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		// Get the GraphQL error
		gqlErr := graphql.DefaultErrorPresenter(ctx, err)

		// Capture error to Sentry with context
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub()
		}

		hub.WithScope(func(scope *sentry.Scope) {
			// Add GraphQL context
			if reqCtx := graphql.GetRequestContext(ctx); reqCtx != nil {
				scope.SetTag("graphql.operation", reqCtx.OperationName)
				// Complexity is calculated per operation, not a limit on the context

				// Add operation type
				if reqCtx.Doc != nil && len(reqCtx.Doc.Operations) > 0 {
					for _, op := range reqCtx.Doc.Operations {
						scope.SetTag("graphql.type", string(op.Operation))
						break
					}
				}

				// Add variables (filtered for sensitive data)
				if reqCtx.Variables != nil {
					filteredVars := filterSensitiveVariables(reqCtx.Variables)
					scope.SetContext("graphql", map[string]interface{}{
						"variables": filteredVars,
						"query":     reqCtx.RawQuery,
					})
				}
			}

			// Add path context
			if gqlErr.Path != nil {
				scope.SetTag("graphql.path", gqlErr.Path.String())
			}

			// Set error level based on error type
			level := sentry.LevelError
			if isUserError(err) {
				level = sentry.LevelWarning
			}
			scope.SetLevel(level)

			// Capture the exception
			hub.CaptureException(err)
		})

		return gqlErr
	}
}

// SentryRecoveryMiddleware creates GraphQL middleware for panic recovery
func SentryRecoveryMiddleware() graphql.RecoverFunc {
	return func(ctx context.Context, err interface{}) error {
		// Capture panic to Sentry
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub()
		}

		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)
			scope.SetTag("panic", "true")

			// Add GraphQL context if available
			if reqCtx := graphql.GetRequestContext(ctx); reqCtx != nil {
				scope.SetTag("graphql.operation", reqCtx.OperationName)
				scope.SetContext("graphql", map[string]interface{}{
					"query": reqCtx.RawQuery,
				})
			}

			// Recover and capture
			hub.Recover(err)
		})

		// Return error to GraphQL
		return fmt.Errorf("internal server error: %v", err)
	}
}

// SentryTransactionMiddleware creates middleware for performance monitoring
func SentryTransactionMiddleware() graphql.ResponseMiddleware {
	return func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		// Start transaction
		reqCtx := graphql.GetRequestContext(ctx)
		if reqCtx == nil {
			return next(ctx)
		}

		// Create transaction name
		txName := "GraphQL"
		if reqCtx.OperationName != "" {
			txName = fmt.Sprintf("GraphQL: %s", reqCtx.OperationName)
		}

		// Start Sentry transaction
		span := sentry.StartSpan(ctx, "graphql.execute")
		span.Description = txName
		defer span.Finish()

		// Update context with span
		ctx = span.Context()

		// Add operation details
		span.SetTag("graphql.operation", reqCtx.OperationName)
		if reqCtx.Doc != nil && len(reqCtx.Doc.Operations) > 0 {
			for _, op := range reqCtx.Doc.Operations {
				span.SetTag("graphql.type", string(op.Operation))
				break
			}
		}

		// Execute next handler
		response := next(ctx)

		// Record errors if any
		if len(response.Errors) > 0 {
			span.Status = sentry.SpanStatusInternalError
			for _, err := range response.Errors {
				span.SetTag("graphql.error", err.Message)
				break // Only record first error as tag
			}
		} else {
			span.Status = sentry.SpanStatusOK
		}

		return response
	}
}

// filterSensitiveVariables filters sensitive data from GraphQL variables
func filterSensitiveVariables(variables map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	sensitiveKeys := []string{"password", "token", "secret", "key", "authorization"}

	for key, value := range variables {
		isSensitive := false
		lowerKey := toLower(key)

		for _, sensitive := range sensitiveKeys {
			if contains(lowerKey, sensitive) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			filtered[key] = "[FILTERED]"
		} else {
			filtered[key] = value
		}
	}

	return filtered
}

// isUserError checks if error is a user error (validation, auth, etc)
func isUserError(err error) bool {
	// Check for common user error types
	switch err.Error() {
	case "unauthorized", "forbidden", "invalid input", "validation error":
		return true
	}

	// Check error message patterns
	errMsg := err.Error()
	userErrorPatterns := []string{
		"invalid", "unauthorized", "forbidden",
		"not found", "already exists", "duplicate",
		"validation", "bad request",
	}

	lowerMsg := toLower(errMsg)
	for _, pattern := range userErrorPatterns {
		if contains(lowerMsg, pattern) {
			return true
		}
	}

	return false
}

// Helper functions
func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WithSentryUser adds user information to Sentry context
func WithSentryUser(ctx context.Context, userID, username, email string) context.Context {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{
			ID:       userID,
			Username: username,
			Email:    email,
		})
	})

	return ctx
}

// AddSentryBreadcrumb adds a breadcrumb to the current Sentry scope
func AddSentryBreadcrumb(ctx context.Context, message string, data map[string]interface{}) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}

	hub.AddBreadcrumb(&sentry.Breadcrumb{
		Message:   message,
		Level:     sentry.LevelInfo,
		Timestamp: time.Now(),
		Data:      data,
	}, nil)
}
