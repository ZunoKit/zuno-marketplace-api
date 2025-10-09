package directives

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

// RateLimiter implements GraphQL rate limiting directive
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*TokenBucket
}

// TokenBucket implements token bucket algorithm
type TokenBucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastFill time.Time
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Directive implements the @rateLimit directive
func (rl *RateLimiter) Directive(ctx context.Context, obj interface{}, next graphql.Resolver, limit int, window int) (interface{}, error) {
	// Get user identifier
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, fmt.Errorf("rate limiting requires authentication")
	}

	// Create bucket key
	fieldCtx := graphql.GetFieldContext(ctx)
	bucketKey := fmt.Sprintf("%s:%s.%s", user.ID, fieldCtx.Object, fieldCtx.Field.Name)

	// Check rate limit
	if !rl.allow(bucketKey, limit, window) {
		return nil, fmt.Errorf("rate limit exceeded: max %d requests per %d seconds", limit, window)
	}

	return next(ctx)
}

// allow checks if request is allowed
func (rl *RateLimiter) allow(key string, limit int, window int) bool {
	rl.mu.Lock()
	bucket, exists := rl.buckets[key]
	if !exists {
		// Create new bucket
		bucket = &TokenBucket{
			capacity: float64(limit),
			tokens:   float64(limit),
			rate:     float64(limit) / float64(window),
			lastFill: time.Now(),
		}
		rl.buckets[key] = bucket
	}
	rl.mu.Unlock()

	return bucket.Allow(1)
}

// Allow checks if n tokens are available
func (tb *TokenBucket) Allow(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.tokens = min(tb.capacity, tb.tokens+elapsed*tb.rate)
	tb.lastFill = now

	// Check if enough tokens
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}

	return false
}

// cleanup removes unused buckets
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		// Remove buckets not used for 10 minutes
		for key, bucket := range rl.buckets {
			bucket.mu.Lock()
			if now.Sub(bucket.lastFill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
			bucket.mu.Unlock()
		}

		rl.mu.Unlock()
	}
}

// min returns minimum of two floats
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// IPRateLimiter implements IP-based rate limiting
type IPRateLimiter struct {
	mu     sync.RWMutex
	limits map[string]*IPLimit
	global RateLimit
}

// IPLimit tracks rate limit for an IP
type IPLimit struct {
	requests []time.Time
	blocked  time.Time
}

// RateLimit configuration
type RateLimit struct {
	Requests int
	Window   time.Duration
	Blockage time.Duration
}

// NewIPRateLimiter creates IP-based rate limiter
func NewIPRateLimiter(requests int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		limits: make(map[string]*IPLimit),
		global: RateLimit{
			Requests: requests,
			Window:   window,
			Blockage: 15 * time.Minute,
		},
	}
}

// Allow checks if IP is allowed to make request
func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create limit for IP
	limit, exists := rl.limits[ip]
	if !exists {
		limit = &IPLimit{
			requests: make([]time.Time, 0, rl.global.Requests),
		}
		rl.limits[ip] = limit
	}

	// Check if IP is blocked
	if !limit.blocked.IsZero() && now.Before(limit.blocked.Add(rl.global.Blockage)) {
		return false
	}

	// Remove old requests outside window
	cutoff := now.Add(-rl.global.Window)
	validRequests := make([]time.Time, 0, len(limit.requests))
	for _, reqTime := range limit.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	limit.requests = validRequests

	// Check if limit exceeded
	if len(limit.requests) >= rl.global.Requests {
		limit.blocked = now
		return false
	}

	// Add current request
	limit.requests = append(limit.requests, now)
	return true
}

// QueryComplexityLimiter limits query complexity
type QueryComplexityLimiter struct {
	maxComplexity int
}

// NewQueryComplexityLimiter creates complexity limiter
func NewQueryComplexityLimiter(maxComplexity int) *QueryComplexityLimiter {
	return &QueryComplexityLimiter{
		maxComplexity: maxComplexity,
	}
}

// Validate checks if query complexity is within limits
func (qcl *QueryComplexityLimiter) Validate(ctx context.Context, complexity int) error {
	if complexity > qcl.maxComplexity {
		return fmt.Errorf("query complexity %d exceeds maximum %d", complexity, qcl.maxComplexity)
	}
	return nil
}
