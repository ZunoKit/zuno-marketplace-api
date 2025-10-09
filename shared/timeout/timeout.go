package timeout

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	Default     time.Duration
	Database    time.Duration
	Redis       time.Duration
	HTTP        time.Duration
	GRPC        time.Duration
	Blockchain  time.Duration
	FileUpload  time.Duration
	LongRunning time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Default:     30 * time.Second,
		Database:    5 * time.Second,
		Redis:       2 * time.Second,
		HTTP:        30 * time.Second,
		GRPC:        10 * time.Second,
		Blockchain:  60 * time.Second,
		FileUpload:  5 * time.Minute,
		LongRunning: 10 * time.Minute,
	}
}

// WithTimeout creates a context with timeout
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second // Default timeout
	}
	return context.WithTimeout(ctx, timeout)
}

// WithDeadline creates a context with deadline
func WithDeadline(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, deadline)
}

// TimeoutInterceptor is a gRPC unary interceptor that adds timeouts
func TimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip timeout for specific methods that need longer execution
		if shouldSkipTimeout(info.FullMethod) {
			return handler(ctx, req)
		}

		// Get method-specific timeout
		methodTimeout := getMethodTimeout(info.FullMethod, timeout)

		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, methodTimeout)
		defer cancel()

		// Channel to handle response
		type responseError struct {
			resp interface{}
			err  error
		}
		responseChan := make(chan responseError, 1)

		// Execute handler in goroutine
		go func() {
			resp, err := handler(timeoutCtx, req)
			responseChan <- responseError{resp, err}
		}()

		// Wait for response or timeout
		select {
		case r := <-responseChan:
			return r.resp, r.err
		case <-timeoutCtx.Done():
			err := status.Errorf(codes.DeadlineExceeded,
				"request timeout exceeded (%v) for method %s", methodTimeout, info.FullMethod)
			return nil, err
		}
	}
}

// StreamTimeoutInterceptor is a gRPC stream interceptor that adds timeouts
func StreamTimeoutInterceptor(timeout time.Duration) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Skip timeout for specific methods
		if shouldSkipTimeout(info.FullMethod) {
			return handler(srv, ss)
		}

		// Get method-specific timeout (streams usually need longer)
		methodTimeout := getMethodTimeout(info.FullMethod, timeout*3)

		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ss.Context(), methodTimeout)
		defer cancel()

		// Wrap stream with timeout context
		wrappedStream := &timeoutServerStream{
			ServerStream: ss,
			ctx:          timeoutCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// timeoutServerStream wraps grpc.ServerStream with timeout context
type timeoutServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *timeoutServerStream) Context() context.Context {
	return s.ctx
}

// shouldSkipTimeout checks if a method should skip timeout
func shouldSkipTimeout(method string) bool {
	skipMethods := []string{
		"/FileUpload/Upload",
		"/Blockchain/WaitForTransaction",
		"/Subscription/Subscribe",
	}

	for _, skipMethod := range skipMethods {
		if method == skipMethod {
			return true
		}
	}

	return false
}

// getMethodTimeout returns method-specific timeout
func getMethodTimeout(method string, defaultTimeout time.Duration) time.Duration {
	// Define custom timeouts for specific methods
	methodTimeouts := map[string]time.Duration{
		"/AuthService/GetNonce":         5 * time.Second,
		"/AuthService/VerifySiwe":       10 * time.Second,
		"/AuthService/RefreshSession":   5 * time.Second,
		"/UserService/GetUser":          5 * time.Second,
		"/UserService/CreateUser":       10 * time.Second,
		"/WalletService/LinkWallet":     10 * time.Second,
		"/MediaService/UploadMedia":     5 * time.Minute,
		"/ChainService/GetGasPrice":     15 * time.Second,
		"/OrchestratorService/TrackTx":  30 * time.Second,
		"/IndexerService/ProcessBlocks": 2 * time.Minute,
		"/CatalogService/SearchNFTs":    15 * time.Second,
	}

	if timeout, ok := methodTimeouts[method]; ok {
		return timeout
	}

	return defaultTimeout
}

// TimeoutMiddleware is an HTTP middleware that adds request timeouts
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Channel to track if handler completes
			done := make(chan struct{})

			// Run handler in goroutine
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				// Handler completed successfully
			case <-ctx.Done():
				// Timeout occurred
				http.Error(w, "Request timeout", http.StatusRequestTimeout)
			}
		})
	}
}

// DatabaseTimeout wraps database operations with timeout
func DatabaseTimeout(ctx context.Context, config *TimeoutConfig, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Database)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- fn(timeoutCtx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("database operation timeout after %v", config.Database)
	}
}

// RedisTimeout wraps Redis operations with timeout
func RedisTimeout(ctx context.Context, config *TimeoutConfig, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Redis)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- fn(timeoutCtx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("redis operation timeout after %v", config.Redis)
	}
}

// BlockchainTimeout wraps blockchain operations with timeout
func BlockchainTimeout(ctx context.Context, config *TimeoutConfig, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Blockchain)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- fn(timeoutCtx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("blockchain operation timeout after %v", config.Blockchain)
	}
}

// TimeoutTracker tracks operation timeouts for monitoring
type TimeoutTracker struct {
	operations map[string]*OperationStats
}

// OperationStats holds timeout statistics for an operation
type OperationStats struct {
	TotalCalls    int64
	TimeoutCount  int64
	SuccessCount  int64
	TotalDuration time.Duration
	MaxDuration   time.Duration
	LastTimeout   time.Time
}

// NewTimeoutTracker creates a new timeout tracker
func NewTimeoutTracker() *TimeoutTracker {
	return &TimeoutTracker{
		operations: make(map[string]*OperationStats),
	}
}

// Track tracks an operation execution
func (t *TimeoutTracker) Track(operation string, duration time.Duration, timedOut bool) {
	stats, ok := t.operations[operation]
	if !ok {
		stats = &OperationStats{}
		t.operations[operation] = stats
	}

	stats.TotalCalls++
	stats.TotalDuration += duration

	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}

	if timedOut {
		stats.TimeoutCount++
		stats.LastTimeout = time.Now()
	} else {
		stats.SuccessCount++
	}
}

// GetStats returns statistics for an operation
func (t *TimeoutTracker) GetStats(operation string) *OperationStats {
	return t.operations[operation]
}

// GetAllStats returns all operation statistics
func (t *TimeoutTracker) GetAllStats() map[string]*OperationStats {
	return t.operations
}
