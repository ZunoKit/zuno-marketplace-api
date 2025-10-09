package middleware

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*clientInfo
	rate            int           // requests per window
	window          time.Duration // time window
	cleanupInterval time.Duration
	done            chan struct{} // Channel to signal goroutine shutdown
	closed          bool          // Flag to prevent multiple closes
}

type clientInfo struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:         make(map[string]*clientInfo),
		rate:            rate,
		window:          window,
		cleanupInterval: window * 2,
		done:            make(chan struct{}),
		closed:          false,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// UnaryInterceptor returns a gRPC unary interceptor for rate limiting
func (rl *RateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		clientID := rl.getClientID(ctx)

		if !rl.allow(clientID) {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded, please try again later")
		}

		return handler(ctx, req)
	}
}

// getClientID extracts client identifier from context
func (rl *RateLimiter) getClientID(ctx context.Context) string {
	// Try to get from metadata (e.g., API key, user ID)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// Check for user ID in metadata
		if userIDs := md.Get("user-id"); len(userIDs) > 0 {
			return "user:" + userIDs[0]
		}

		// Check for API key
		if apiKeys := md.Get("api-key"); len(apiKeys) > 0 {
			return "api:" + apiKeys[0]
		}
	}

	// Fall back to peer address
	if p, ok := peer.FromContext(ctx); ok {
		return "peer:" + p.Addr.String()
	}

	return "unknown"
}

// allow checks if a request from clientID is allowed
func (rl *RateLimiter) allow(clientID string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		// New client
		rl.clients[clientID] = &clientInfo{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Check if window has expired
	if now.Sub(client.windowStart) > rl.window {
		// Reset window
		client.count = 1
		client.windowStart = now
		return true
	}

	// Check rate limit
	if client.count >= rl.rate {
		return false
	}

	client.count++
	return true
}

// cleanup removes old client entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.mu.Lock()

			for clientID, client := range rl.clients {
				if now.Sub(client.windowStart) > rl.window*2 {
					delete(rl.clients, clientID)
				}
			}

			rl.mu.Unlock()
		case <-rl.done:
			// Gracefully shutdown cleanup goroutine
			return
		}
	}
}

// Close gracefully shuts down the rate limiter
func (rl *RateLimiter) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.closed {
		close(rl.done)
		rl.closed = true
		// Clear all clients
		rl.clients = make(map[string]*clientInfo)
	}
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled bool
	Rate    int           // requests per window
	Window  time.Duration // time window
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled: true,
		Rate:    100,         // 100 requests
		Window:  time.Minute, // per minute
	}
}

// AuthServiceRateLimitConfig returns rate limit config for auth service
func AuthServiceRateLimitConfig() map[string]RateLimitConfig {
	return map[string]RateLimitConfig{
		"/auth.AuthService/GetNonce": {
			Enabled: true,
			Rate:    10,
			Window:  time.Minute,
		},
		"/auth.AuthService/VerifySiwe": {
			Enabled: true,
			Rate:    5,
			Window:  time.Minute,
		},
		"/auth.AuthService/RefreshSession": {
			Enabled: true,
			Rate:    20,
			Window:  time.Minute,
		},
	}
}

// MethodRateLimiter provides per-method rate limiting
type MethodRateLimiter struct {
	limiters map[string]*RateLimiter
	configs  map[string]RateLimitConfig
}

// NewMethodRateLimiter creates a new method-specific rate limiter
func NewMethodRateLimiter(configs map[string]RateLimitConfig) *MethodRateLimiter {
	limiters := make(map[string]*RateLimiter)

	for method, config := range configs {
		if config.Enabled {
			limiters[method] = NewRateLimiter(config.Rate, config.Window)
		}
	}

	return &MethodRateLimiter{
		limiters: limiters,
		configs:  configs,
	}
}

// UnaryInterceptor returns a gRPC unary interceptor for method-specific rate limiting
func (mrl *MethodRateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if rate limiting is enabled for this method
		limiter, exists := mrl.limiters[info.FullMethod]
		if !exists {
			// No rate limiting for this method
			return handler(ctx, req)
		}

		clientID := limiter.getClientID(ctx)
		if !limiter.allow(clientID) {
			config := mrl.configs[info.FullMethod]
			return nil, status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded for %s: maximum %d requests per %v",
				info.FullMethod, config.Rate, config.Window)
		}

		return handler(ctx, req)
	}
}
