package grpcclients

import (
	"context"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ResilientClient wraps a gRPC connection with circuit breaker functionality
type ResilientClient struct {
	serviceName    string
	conn           *grpc.ClientConn
	circuitBreaker *resilience.CircuitBreaker
	logger         *logging.Logger
}

// NewResilientClient creates a new gRPC client with circuit breaker protection
func NewResilientClient(serviceName, url string, logger *logging.Logger) (*ResilientClient, error) {
	// Create circuit breaker for this service
	cb := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
		Name:             serviceName,
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange: func(name string, from, to resilience.State) {
			logger.WithFields(map[string]interface{}{
				"service": name,
				"from":    from.String(),
				"to":      to.String(),
			}).Info("Circuit breaker state changed")
		},
	})

	// Create gRPC connection with circuit breaker interceptor
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(createUnaryInterceptor(cb, serviceName, logger)),
		grpc.WithStreamInterceptor(createStreamInterceptor(cb, serviceName, logger)),
	}

	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s service: %w", serviceName, err)
	}

	return &ResilientClient{
		serviceName:    serviceName,
		conn:           conn,
		circuitBreaker: cb,
		logger:         logger,
	}, nil
}

// GetConnection returns the underlying gRPC connection
func (rc *ResilientClient) GetConnection() *grpc.ClientConn {
	return rc.conn
}

// GetCircuitBreaker returns the circuit breaker for monitoring
func (rc *ResilientClient) GetCircuitBreaker() *resilience.CircuitBreaker {
	return rc.circuitBreaker
}

// GetStats returns circuit breaker statistics
func (rc *ResilientClient) GetStats() resilience.CircuitBreakerStats {
	return rc.circuitBreaker.GetStats()
}

// Close closes the gRPC connection
func (rc *ResilientClient) Close() error {
	if rc.conn != nil {
		return rc.conn.Close()
	}
	return nil
}

// createUnaryInterceptor creates a unary interceptor with circuit breaker
func createUnaryInterceptor(cb *resilience.CircuitBreaker, serviceName string, logger *logging.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Execute with circuit breaker
		err := cb.Execute(ctx, func(execCtx context.Context) error {
			return invoker(execCtx, method, req, reply, cc, opts...)
		})

		// Handle circuit breaker open error
		if err != nil && err.Error() == fmt.Sprintf("circuit breaker '%s' is OPEN", serviceName) {
			logger.WithFields(map[string]interface{}{
				"service": serviceName,
				"method":  method,
			}).Warn("Circuit breaker is open")
			return status.Errorf(codes.Unavailable, "service %s is temporarily unavailable", serviceName)
		}

		// Check if error should trip the circuit breaker
		if err != nil {
			// Only count certain errors as failures for circuit breaker
			if shouldTripCircuitBreaker(err) {
				return err
			}
			// For other errors, reset failure count to avoid false positives
			cb.Reset()
		}

		return err
	}
}

// createStreamInterceptor creates a stream interceptor with circuit breaker
func createStreamInterceptor(cb *resilience.CircuitBreaker, serviceName string, logger *logging.Logger) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		var stream grpc.ClientStream
		var streamErr error

		// Execute with circuit breaker
		err := cb.Execute(ctx, func(execCtx context.Context) error {
			stream, streamErr = streamer(execCtx, desc, cc, method, opts...)
			return streamErr
		})

		// Handle circuit breaker open error
		if err != nil && err.Error() == fmt.Sprintf("circuit breaker '%s' is OPEN", serviceName) {
			logger.WithFields(map[string]interface{}{
				"service": serviceName,
				"method":  method,
			}).Warn("Circuit breaker is open for stream")
			return nil, status.Errorf(codes.Unavailable, "service %s is temporarily unavailable", serviceName)
		}

		// Check if error should trip the circuit breaker
		if err != nil && shouldTripCircuitBreaker(err) {
			return nil, err
		}

		return stream, err
	}
}

// shouldTripCircuitBreaker determines if an error should count as a circuit breaker failure
func shouldTripCircuitBreaker(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		// Non-gRPC errors should trip the breaker
		return true
	}

	// Only certain error codes should trip the circuit breaker
	switch st.Code() {
	case codes.Unavailable, codes.Internal, codes.Unknown, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	case codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.AlreadyExists,
		codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition,
		codes.Aborted, codes.OutOfRange, codes.Unimplemented, codes.DataLoss:
		// These are typically client errors or expected errors, don't trip the breaker
		return false
	default:
		return true
	}
}

// CircuitBreakerMetrics provides metrics for circuit breakers
type CircuitBreakerMetrics struct {
	clients map[string]*ResilientClient
}

// NewCircuitBreakerMetrics creates a new metrics collector
func NewCircuitBreakerMetrics() *CircuitBreakerMetrics {
	return &CircuitBreakerMetrics{
		clients: make(map[string]*ResilientClient),
	}
}

// RegisterClient registers a client for metrics collection
func (m *CircuitBreakerMetrics) RegisterClient(name string, client *ResilientClient) {
	m.clients[name] = client
}

// GetAllStats returns stats for all registered clients
func (m *CircuitBreakerMetrics) GetAllStats() map[string]resilience.CircuitBreakerStats {
	stats := make(map[string]resilience.CircuitBreakerStats)
	for name, client := range m.clients {
		stats[name] = client.GetStats()
	}
	return stats
}
