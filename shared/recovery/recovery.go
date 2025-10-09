package recovery

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PanicHandler handles panic recovery
type PanicHandler struct {
	onPanic      func(recovered interface{}, stack []byte)
	logStack     bool
	returnErrors bool
}

// Option configures PanicHandler
type Option func(*PanicHandler)

// WithPanicCallback sets a callback for when panic occurs
func WithPanicCallback(fn func(recovered interface{}, stack []byte)) Option {
	return func(ph *PanicHandler) {
		ph.onPanic = fn
	}
}

// WithStackLogging enables stack trace logging
func WithStackLogging(enabled bool) Option {
	return func(ph *PanicHandler) {
		ph.logStack = enabled
	}
}

// WithErrorReturn enables returning error details
func WithErrorReturn(enabled bool) Option {
	return func(ph *PanicHandler) {
		ph.returnErrors = enabled
	}
}

// NewPanicHandler creates a new panic handler
func NewPanicHandler(opts ...Option) *PanicHandler {
	ph := &PanicHandler{
		logStack:     true,
		returnErrors: false,
	}

	for _, opt := range opts {
		opt(ph)
	}

	return ph
}

// UnaryServerInterceptor returns a gRPC unary interceptor for panic recovery
func (ph *PanicHandler) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = ph.handlePanic(ctx, r, info.FullMethod)
			}
		}()

		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a gRPC stream interceptor for panic recovery
func (ph *PanicHandler) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = ph.handlePanic(stream.Context(), r, info.FullMethod)
			}
		}()

		return handler(srv, stream)
	}
}

// HTTPMiddleware returns an HTTP middleware for panic recovery
func (ph *PanicHandler) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				ph.handleHTTPPanic(w, r, rec)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// handlePanic handles a panic in gRPC context
func (ph *PanicHandler) handlePanic(ctx context.Context, recovered interface{}, method string) error {
	stack := debug.Stack()

	// Log the panic
	if ph.logStack {
		fmt.Printf("PANIC in %s: %v\n%s", method, recovered, stack)
	}

	// Call panic callback if set
	if ph.onPanic != nil {
		ph.onPanic(recovered, stack)
	}

	// Send to Sentry if configured
	if sentry.CurrentHub() != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)
			scope.SetContext("panic", map[string]interface{}{
				"method":    method,
				"recovered": recovered,
				"stack":     string(stack),
			})
			sentry.CaptureException(fmt.Errorf("panic: %v", recovered))
		})
	}

	// Return appropriate error
	if ph.returnErrors {
		return status.Errorf(codes.Internal, "internal server error: %v", recovered)
	}
	return status.Error(codes.Internal, "internal server error")
}

// handleHTTPPanic handles a panic in HTTP context
func (ph *PanicHandler) handleHTTPPanic(w http.ResponseWriter, r *http.Request, recovered interface{}) {
	stack := debug.Stack()

	// Log the panic
	if ph.logStack {
		fmt.Printf("HTTP PANIC at %s %s: %v\n%s", r.Method, r.URL.Path, recovered, stack)
	}

	// Call panic callback if set
	if ph.onPanic != nil {
		ph.onPanic(recovered, stack)
	}

	// Send to Sentry if configured
	if sentry.CurrentHub() != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)
			scope.SetRequest(r)
			scope.SetContext("panic", map[string]interface{}{
				"path":      r.URL.Path,
				"method":    r.Method,
				"recovered": recovered,
				"stack":     string(stack),
			})
			sentry.CaptureException(fmt.Errorf("http panic: %v", recovered))
		})
	}

	// Return error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	if ph.returnErrors {
		fmt.Fprintf(w, `{"error":"internal server error: %v"}`, recovered)
	} else {
		fmt.Fprint(w, `{"error":"internal server error"}`)
	}
}

// SafeGo runs a goroutine with panic recovery
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				fmt.Printf("PANIC in goroutine: %v\n%s", r, stack)

				// Send to Sentry
				if sentry.CurrentHub() != nil {
					sentry.CaptureException(fmt.Errorf("goroutine panic: %v", r))
				}
			}
		}()

		fn()
	}()
}

// SafeGoWithContext runs a goroutine with panic recovery and context
func SafeGoWithContext(ctx context.Context, fn func(context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				fmt.Printf("PANIC in goroutine: %v\n%s", r, stack)

				// Send to Sentry with context
				if sentry.CurrentHub() != nil {
					sentry.WithScope(func(scope *sentry.Scope) {
						scope.SetContext("goroutine", map[string]interface{}{
							"recovered": r,
							"stack":     string(stack),
						})
						sentry.CaptureException(fmt.Errorf("goroutine panic: %v", r))
					})
				}
			}
		}()

		fn(ctx)
	}()
}
