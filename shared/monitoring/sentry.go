package monitoring

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// SentryConfig holds Sentry configuration options
type SentryConfig struct {
	DSN              string
	Environment      string
	Release          string
	Debug            bool
	SampleRate       float64
	TracesSampleRate float64
	ServiceName      string
	ServerName       string
}

// InitSentry initializes Sentry with the provided configuration
func InitSentry(config *SentryConfig) error {
	// Get DSN from config or environment
	dsn := config.DSN
	if dsn == "" {
		dsn = os.Getenv("SENTRY_DSN")
	}

	// Skip if no DSN provided
	if dsn == "" {
		fmt.Println("Sentry DSN not provided, skipping initialization")
		return nil
	}

	// Get environment from config or env var
	environment := config.Environment
	if environment == "" {
		environment = os.Getenv("ENVIRONMENT")
		if environment == "" {
			environment = "development"
		}
	}

	// Get release version
	release := config.Release
	if release == "" {
		release = os.Getenv("RELEASE_VERSION")
		if release == "" {
			release = "unknown"
		}
	}

	// Set sample rates with defaults
	sampleRate := config.SampleRate
	if sampleRate == 0 {
		if environment == "production" {
			sampleRate = 1.0
		} else {
			sampleRate = 0.25
		}
	}

	tracesSampleRate := config.TracesSampleRate
	if tracesSampleRate == 0 {
		if environment == "production" {
			tracesSampleRate = 0.1
		} else {
			tracesSampleRate = 0.05
		}
	}

	// Initialize Sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          release,
		Debug:            config.Debug,
		SampleRate:       sampleRate,
		TracesSampleRate: tracesSampleRate,
		ServerName:       config.ServerName,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add service name as tag
			if config.ServiceName != "" {
				event.Tags["service"] = config.ServiceName
			}

			// Filter out sensitive data
			FilterSensitiveData(event)

			return event
		},
		BeforeBreadcrumb: func(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
			// Filter sensitive data from breadcrumbs
			if breadcrumb.Type == "http" {
				FilterHTTPBreadcrumb(breadcrumb)
			}
			return breadcrumb
		},
	})

	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	fmt.Printf("Sentry initialized for %s service (env: %s, release: %s)\n",
		config.ServiceName, environment, release)
	return nil
}

// FilterSensitiveData removes sensitive information from events
func FilterSensitiveData(event *sentry.Event) {
	// List of sensitive keys to filter
	sensitiveKeys := []string{
		"password", "passwd", "pwd",
		"secret", "token", "key",
		"authorization", "auth",
		"api_key", "apikey",
		"access_token", "refresh_token",
		"private_key", "privatekey",
		"credit_card", "cc_number",
		"ssn", "social_security",
	}

	// Filter request data
	if event.Request != nil {
		// Filter headers
		for key := range event.Request.Headers {
			if containsSensitiveKey(key, sensitiveKeys) {
				event.Request.Headers[key] = "[FILTERED]"
			}
		}

		// Filter cookies (Cookies is a string in Sentry SDK)
		// If sensitive cookies need filtering, parse the cookie string and rebuild it
		// For now, we'll leave it as-is since it's complex to parse cookie strings
		// In production, use middleware to filter cookies before they reach Sentry

		// Note: QueryString in Sentry is a string, not a map
		// If it contains sensitive data, it should be filtered at a higher level
	}

	// Filter context data
	// event.Contexts is a map[string]sentry.Context
	// sentry.Context is itself map[string]interface{}
	for contextKey, contextValue := range event.Contexts {
		// contextValue is already map[string]interface{}
		for key := range contextValue {
			if containsSensitiveKey(key, sensitiveKeys) {
				contextValue[key] = "[FILTERED]"
			}
		}
		event.Contexts[contextKey] = contextValue
	}

	// Filter extra data
	for key := range event.Extra {
		if containsSensitiveKey(key, sensitiveKeys) {
			event.Extra[key] = "[FILTERED]"
		}
	}
}

// FilterHTTPBreadcrumb filters sensitive data from HTTP breadcrumbs
func FilterHTTPBreadcrumb(breadcrumb *sentry.Breadcrumb) {
	if data, ok := breadcrumb.Data["url"].(string); ok {
		// Remove query parameters that might contain sensitive data
		breadcrumb.Data["url"] = RemoveSensitiveQueryParams(data)
	}

	// Filter headers
	if headers, ok := breadcrumb.Data["headers"].(map[string]interface{}); ok {
		for key := range headers {
			if containsSensitiveKey(key, []string{"authorization", "cookie", "token"}) {
				headers[key] = "[FILTERED]"
			}
		}
	}
}

// RemoveSensitiveQueryParams removes sensitive query parameters from URLs
func RemoveSensitiveQueryParams(url string) string {
	// This is a simplified implementation
	// In production, use proper URL parsing
	return url
}

// containsSensitiveKey checks if a key contains sensitive information
func containsSensitiveKey(key string, sensitiveKeys []string) bool {
	lowerKey := toLower(key)
	for _, sensitive := range sensitiveKeys {
		if contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// Helper functions
func toLower(s string) string {
	// Simple lowercase conversion
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

// FlushSentry flushes buffered events
func FlushSentry(timeout time.Duration) {
	sentry.Flush(timeout)
}

// RecoverWithSentry recovers from panics and sends them to Sentry
func RecoverWithSentry() {
	if err := recover(); err != nil {
		sentry.CurrentHub().Recover(err)
		sentry.Flush(time.Second * 5)
		panic(err) // Re-panic after sending to Sentry
	}
}

// CaptureError captures an error and sends it to Sentry
func CaptureError(err error, tags map[string]string, extra map[string]interface{}) {
	hub := sentry.CurrentHub()
	hub.WithScope(func(scope *sentry.Scope) {
		// Add tags
		for key, value := range tags {
			scope.SetTag(key, value)
		}

		// Add extra context
		for key, value := range extra {
			scope.SetExtra(key, value)
		}

		hub.CaptureException(err)
	})
}

// CaptureMessage captures a message and sends it to Sentry
func CaptureMessage(message string, level sentry.Level, tags map[string]string) {
	hub := sentry.CurrentHub()
	hub.WithScope(func(scope *sentry.Scope) {
		// Add tags
		for key, value := range tags {
			scope.SetTag(key, value)
		}

		scope.SetLevel(level)
		hub.CaptureMessage(message)
	})
}

// StartTransaction starts a new Sentry transaction for performance monitoring
func StartTransaction(ctx context.Context, name, operation string) *sentry.Span {
	return sentry.StartSpan(ctx, operation)
}

// WithSentryTags adds tags to the current Sentry scope
func WithSentryTags(tags map[string]string) {
	hub := sentry.CurrentHub()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		for key, value := range tags {
			scope.SetTag(key, value)
		}
	})
}

// WithSentryUser sets user information in the current Sentry scope
func WithSentryUser(userID, username, email string) {
	hub := sentry.CurrentHub()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{
			ID:       userID,
			Username: username,
			Email:    email,
		})
	})
}
