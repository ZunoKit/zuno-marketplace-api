package errors

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// Client errors (4xx equivalent)
	ErrorTypeNotFound     ErrorType = "NOT_FOUND"
	ErrorTypeInvalidInput ErrorType = "INVALID_INPUT"
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"
	ErrorTypeForbidden    ErrorType = "FORBIDDEN"
	ErrorTypeConflict     ErrorType = "CONFLICT"
	ErrorTypeRateLimited  ErrorType = "RATE_LIMITED"
	ErrorTypePrecondition ErrorType = "PRECONDITION_FAILED"

	// Server errors (5xx equivalent)
	ErrorTypeInternal       ErrorType = "INTERNAL"
	ErrorTypeUnavailable    ErrorType = "UNAVAILABLE"
	ErrorTypeTimeout        ErrorType = "TIMEOUT"
	ErrorTypeNotImplemented ErrorType = "NOT_IMPLEMENTED"

	// Business logic errors
	ErrorTypeBusinessRule ErrorType = "BUSINESS_RULE"
	ErrorTypeValidation   ErrorType = "VALIDATION"
	ErrorTypeDuplicate    ErrorType = "DUPLICATE"
	ErrorTypeExpired      ErrorType = "EXPIRED"
)

// Error represents a structured error with context
type Error struct {
	Type       ErrorType              `json:"type"`
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Stack      []string               `json:"-"`
	Cause      error                  `json:"-"`
	StatusCode int                    `json:"-"`
	GRPCCode   codes.Code             `json:"-"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithDetails adds details to the error
func (e *Error) WithDetails(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithCause wraps an underlying error
func (e *Error) WithCause(cause error) *Error {
	e.Cause = cause
	return e
}

// captureStack captures the current stack trace
func captureStack() []string {
	var stack []string
	for i := 2; i < 10; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn != nil && !strings.Contains(fn.Name(), "runtime.") {
			stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
		}
	}
	return stack
}

// New creates a new error
func New(errorType ErrorType, code, message string) *Error {
	e := &Error{
		Type:    errorType,
		Code:    code,
		Message: message,
		Stack:   captureStack(),
	}

	// Set default status codes
	switch errorType {
	case ErrorTypeNotFound:
		e.StatusCode = http.StatusNotFound
		e.GRPCCode = codes.NotFound
	case ErrorTypeInvalidInput, ErrorTypeValidation:
		e.StatusCode = http.StatusBadRequest
		e.GRPCCode = codes.InvalidArgument
	case ErrorTypeUnauthorized:
		e.StatusCode = http.StatusUnauthorized
		e.GRPCCode = codes.Unauthenticated
	case ErrorTypeForbidden:
		e.StatusCode = http.StatusForbidden
		e.GRPCCode = codes.PermissionDenied
	case ErrorTypeConflict, ErrorTypeDuplicate:
		e.StatusCode = http.StatusConflict
		e.GRPCCode = codes.AlreadyExists
	case ErrorTypeRateLimited:
		e.StatusCode = http.StatusTooManyRequests
		e.GRPCCode = codes.ResourceExhausted
	case ErrorTypePrecondition, ErrorTypeExpired:
		e.StatusCode = http.StatusPreconditionFailed
		e.GRPCCode = codes.FailedPrecondition
	case ErrorTypeTimeout:
		e.StatusCode = http.StatusRequestTimeout
		e.GRPCCode = codes.DeadlineExceeded
	case ErrorTypeUnavailable:
		e.StatusCode = http.StatusServiceUnavailable
		e.GRPCCode = codes.Unavailable
	case ErrorTypeNotImplemented:
		e.StatusCode = http.StatusNotImplemented
		e.GRPCCode = codes.Unimplemented
	default:
		e.StatusCode = http.StatusInternalServerError
		e.GRPCCode = codes.Internal
	}

	return e
}

// Common error constructors
func NotFound(resource string, id interface{}) *Error {
	return New(ErrorTypeNotFound, "RESOURCE_NOT_FOUND",
		fmt.Sprintf("%s not found", resource)).
		WithDetails("resource", resource).
		WithDetails("id", id)
}

func InvalidInput(field string, reason string) *Error {
	return New(ErrorTypeInvalidInput, "INVALID_INPUT",
		fmt.Sprintf("Invalid input for field '%s': %s", field, reason)).
		WithDetails("field", field).
		WithDetails("reason", reason)
}

func Unauthorized(reason string) *Error {
	return New(ErrorTypeUnauthorized, "UNAUTHORIZED", reason)
}

func Forbidden(resource string, action string) *Error {
	return New(ErrorTypeForbidden, "FORBIDDEN",
		fmt.Sprintf("Forbidden: cannot %s %s", action, resource)).
		WithDetails("resource", resource).
		WithDetails("action", action)
}

func Conflict(resource string, reason string) *Error {
	return New(ErrorTypeConflict, "CONFLICT",
		fmt.Sprintf("Conflict with %s: %s", resource, reason)).
		WithDetails("resource", resource)
}

func Internal(message string) *Error {
	return New(ErrorTypeInternal, "INTERNAL_ERROR", message)
}

func Timeout(operation string) *Error {
	return New(ErrorTypeTimeout, "TIMEOUT",
		fmt.Sprintf("Operation '%s' timed out", operation)).
		WithDetails("operation", operation)
}

func ValidationError(field string, constraint string) *Error {
	return New(ErrorTypeValidation, "VALIDATION_ERROR",
		fmt.Sprintf("Validation failed for '%s': %s", field, constraint)).
		WithDetails("field", field).
		WithDetails("constraint", constraint)
}

func Duplicate(resource string, field string, value interface{}) *Error {
	return New(ErrorTypeDuplicate, "DUPLICATE",
		fmt.Sprintf("%s with %s '%v' already exists", resource, field, value)).
		WithDetails("resource", resource).
		WithDetails("field", field).
		WithDetails("value", value)
}

// ToGRPCError converts to a gRPC status error
func (e *Error) ToGRPCError() error {
	st := status.New(e.GRPCCode, e.Message)

	// Add details if available
	if len(e.Details) > 0 {
		// In production, you'd use proto messages for details
		detailsStr := fmt.Sprintf("%v", e.Details)
		st, _ = st.WithDetails(&detailsStr)
	}

	return st.Err()
}

// FromGRPCError converts from a gRPC status error
func FromGRPCError(err error) *Error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return Internal(err.Error()).WithCause(err)
	}

	var errorType ErrorType
	switch st.Code() {
	case codes.NotFound:
		errorType = ErrorTypeNotFound
	case codes.InvalidArgument:
		errorType = ErrorTypeInvalidInput
	case codes.Unauthenticated:
		errorType = ErrorTypeUnauthorized
	case codes.PermissionDenied:
		errorType = ErrorTypeForbidden
	case codes.AlreadyExists:
		errorType = ErrorTypeConflict
	case codes.ResourceExhausted:
		errorType = ErrorTypeRateLimited
	case codes.FailedPrecondition:
		errorType = ErrorTypePrecondition
	case codes.DeadlineExceeded:
		errorType = ErrorTypeTimeout
	case codes.Unavailable:
		errorType = ErrorTypeUnavailable
	case codes.Unimplemented:
		errorType = ErrorTypeNotImplemented
	default:
		errorType = ErrorTypeInternal
	}

	return New(errorType, string(st.Code()), st.Message()).WithCause(err)
}

// ErrorHandler provides context-aware error handling
type ErrorHandler struct {
	ctx     context.Context
	service string
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(ctx context.Context, service string) *ErrorHandler {
	return &ErrorHandler{
		ctx:     ctx,
		service: service,
	}
}

// Handle processes an error with context
func (h *ErrorHandler) Handle(err error) *Error {
	if err == nil {
		return nil
	}

	// Check if it's already our error type
	if e, ok := err.(*Error); ok {
		return e
	}

	// Check for specific error types
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "not found"):
		return NotFound("resource", "unknown").WithCause(err)
	case strings.Contains(errStr, "duplicate"):
		return Duplicate("resource", "field", "value").WithCause(err)
	case strings.Contains(errStr, "timeout"):
		return Timeout("operation").WithCause(err)
	case strings.Contains(errStr, "unauthorized"):
		return Unauthorized(errStr).WithCause(err)
	case strings.Contains(errStr, "forbidden"):
		return Forbidden("resource", "action").WithCause(err)
	default:
		return Internal(errStr).WithCause(err)
	}
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if e, ok := err.(*Error); ok {
		return e.Type == errorType
	}
	return false
}

// GetCode returns the error code if it's our error type
func GetCode(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return "UNKNOWN"
}
