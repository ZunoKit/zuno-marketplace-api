package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterConfig holds rate limiting configuration
type RateLimiterConfig struct {
	// Requests per second
	RatePerSecond float64
	// Burst size
	Burst int
	// Different limits for different endpoints
	EndpointLimits map[string]*EndpointLimit
	// Different limits for authenticated vs unauthenticated users
	AuthenticatedRatePerSecond   float64
	UnauthenticatedRatePerSecond float64
	// IP-based or user-based limiting
	LimitByIP   bool
	LimitByUser bool
	// Cleanup interval for inactive limiters
	CleanupInterval time.Duration
}

// EndpointLimit defines rate limits for a specific endpoint
type EndpointLimit struct {
	Path          string
	RatePerSecond float64
	Burst         int
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		RatePerSecond:                10,
		Burst:                        20,
		AuthenticatedRatePerSecond:   50,
		UnauthenticatedRatePerSecond: 10,
		LimitByIP:                    true,
		LimitByUser:                  false,
		CleanupInterval:              5 * time.Minute,
		EndpointLimits: map[string]*EndpointLimit{
			"/graphql": {
				Path:          "/graphql",
				RatePerSecond: 30,
				Burst:         60,
			},
			"/upload": {
				Path:          "/upload",
				RatePerSecond: 5,
				Burst:         10,
			},
			"/auth/siwe": {
				Path:          "/auth/siwe",
				RatePerSecond: 5,
				Burst:         10,
			},
		},
	}
}

// RateLimiter implements rate limiting middleware
type RateLimiter struct {
	config       *RateLimiterConfig
	limiters     map[string]*rateLimiterEntry
	mu           sync.RWMutex
	cleanupTimer *time.Timer
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	rl := &RateLimiter{
		config:   config,
		limiters: make(map[string]*rateLimiterEntry),
	}

	// Start cleanup routine
	rl.startCleanup()

	return rl
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get identifier for rate limiting
			identifier := rl.getIdentifier(r)

			// Get or create limiter for this identifier
			limiter := rl.getLimiter(identifier, r)

			// Check if request is allowed
			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GraphQLMiddleware provides rate limiting for GraphQL operations
func (rl *RateLimiter) GraphQLMiddleware(ctx context.Context, operation string) error {
	// Extract request info from context
	identifier := rl.getIdentifierFromContext(ctx)

	// Get appropriate rate limit for GraphQL operations
	limiter := rl.getGraphQLLimiter(identifier, operation)

	// Check if request is allowed
	if !limiter.Allow() {
		return fmt.Errorf("rate limit exceeded for operation: %s", operation)
	}

	return nil
}

// getIdentifier extracts the identifier for rate limiting
func (rl *RateLimiter) getIdentifier(r *http.Request) string {
	var identifier string

	if rl.config.LimitByUser {
		// Extract user ID from context or auth header
		userID := getUserFromRequest(r)
		if userID != "" {
			identifier = "user:" + userID
		}
	}

	if identifier == "" && rl.config.LimitByIP {
		// Fall back to IP-based limiting
		ip := getIPFromRequest(r)
		identifier = "ip:" + ip
	}

	if identifier == "" {
		// Default identifier
		identifier = "global"
	}

	return identifier
}

// getIdentifierFromContext extracts identifier from context
func (rl *RateLimiter) getIdentifierFromContext(ctx context.Context) string {
	// Try to get user ID from context
	if userID, ok := ctx.Value("user_id").(string); ok && rl.config.LimitByUser {
		return "user:" + userID
	}

	// Try to get IP from context
	if ip, ok := ctx.Value("client_ip").(string); ok && rl.config.LimitByIP {
		return "ip:" + ip
	}

	return "global"
}

// getLimiter returns a rate limiter for the given identifier
func (rl *RateLimiter) getLimiter(identifier string, r *http.Request) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[identifier]
	rl.mu.RUnlock()

	if exists {
		// Update last seen time
		rl.mu.Lock()
		entry.lastSeen = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	// Create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := rl.limiters[identifier]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	// Determine rate limit based on endpoint and authentication
	ratePerSecond, burst := rl.getRateLimitForRequest(r)

	limiter := rate.NewLimiter(rate.Limit(ratePerSecond), burst)
	rl.limiters[identifier] = &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// getGraphQLLimiter returns a rate limiter for GraphQL operations
func (rl *RateLimiter) getGraphQLLimiter(identifier string, operation string) *rate.Limiter {
	// Modify identifier to include operation type
	fullIdentifier := identifier + ":graphql:" + operation

	rl.mu.RLock()
	entry, exists := rl.limiters[fullIdentifier]
	rl.mu.RUnlock()

	if exists {
		rl.mu.Lock()
		entry.lastSeen = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	// Create new limiter for GraphQL
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Different rates for different operation types
	var ratePerSecond float64
	var burst int

	switch operation {
	case "mutation":
		ratePerSecond = 10
		burst = 20
	case "subscription":
		ratePerSecond = 5
		burst = 10
	default: // query
		ratePerSecond = 30
		burst = 60
	}

	limiter := rate.NewLimiter(rate.Limit(ratePerSecond), burst)
	rl.limiters[fullIdentifier] = &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// getRateLimitForRequest determines the rate limit for a request
func (rl *RateLimiter) getRateLimitForRequest(r *http.Request) (float64, int) {
	// Check endpoint-specific limits
	if endpointLimit, exists := rl.config.EndpointLimits[r.URL.Path]; exists {
		return endpointLimit.RatePerSecond, endpointLimit.Burst
	}

	// Check if user is authenticated
	if isAuthenticated(r) {
		return rl.config.AuthenticatedRatePerSecond, rl.config.Burst * 2
	}

	// Default to unauthenticated rate
	return rl.config.UnauthenticatedRatePerSecond, rl.config.Burst
}

// startCleanup starts the cleanup routine for inactive limiters
func (rl *RateLimiter) startCleanup() {
	rl.cleanupTimer = time.AfterFunc(rl.config.CleanupInterval, func() {
		rl.cleanup()
		rl.startCleanup() // Reschedule
	})
}

// cleanup removes inactive limiters
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-rl.config.CleanupInterval)

	for identifier, entry := range rl.limiters {
		if entry.lastSeen.Before(threshold) {
			delete(rl.limiters, identifier)
		}
	}
}

// Stop stops the rate limiter and cleanup routine
func (rl *RateLimiter) Stop() {
	if rl.cleanupTimer != nil {
		rl.cleanupTimer.Stop()
	}
}

// getUserFromRequest extracts user ID from request
func getUserFromRequest(r *http.Request) string {
	// Check Authorization header
	if auth := r.Header.Get("Authorization"); auth != "" {
		// Parse JWT or extract user ID
		// This is a simplified version
		return extractUserIDFromAuth(auth)
	}

	// Check context
	if userID, ok := r.Context().Value("user_id").(string); ok {
		return userID
	}

	return ""
}

// getIPFromRequest extracts client IP from request
func getIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := indexOf(xff, ","); idx != -1 {
			return xff[:idx]
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if idx := lastIndexOf(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}

// isAuthenticated checks if request is authenticated
func isAuthenticated(r *http.Request) bool {
	return r.Header.Get("Authorization") != ""
}

// extractUserIDFromAuth extracts user ID from auth header (simplified)
func extractUserIDFromAuth(auth string) string {
	// This would normally parse JWT and extract user ID
	// For now, return a placeholder
	return "authenticated_user"
}

// Helper functions
func indexOf(s, substr string) int {
	for i := 0; i < len(s); i++ {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
