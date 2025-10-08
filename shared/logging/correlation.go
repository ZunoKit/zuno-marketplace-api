package logging

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"
)

// Context keys for correlation IDs
type contextKey string

const (
	CorrelationIDKey  contextKey = "correlation_id"
	RequestIDKey      contextKey = "request_id"
	SessionIDKey      contextKey = "session_id"
	TraceIDKey        contextKey = "trace_id"
	UserIDKey         contextKey = "user_id"
	CorrelationHeader            = "X-Correlation-ID"
	RequestIDHeader              = "X-Request-ID"
)

// CorrelationLogger wraps standard logger with correlation ID support
type CorrelationLogger struct {
	logger *log.Logger
	fields map[string]interface{}
}

// NewCorrelationLogger creates a new correlation logger
func NewCorrelationLogger(logger *log.Logger) *CorrelationLogger {
	if logger == nil {
		logger = log.Default()
	}
	return &CorrelationLogger{
		logger: logger,
		fields: make(map[string]interface{}),
	}
}

// WithContext creates a logger with correlation ID from context
func (cl *CorrelationLogger) WithContext(ctx context.Context) *CorrelationLogger {
	newLogger := &CorrelationLogger{
		logger: cl.logger,
		fields: make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range cl.fields {
		newLogger.fields[k] = v
	}

	// Add correlation IDs from context
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		newLogger.fields["correlation_id"] = correlationID
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		newLogger.fields["request_id"] = requestID
	}

	if sessionID := GetSessionID(ctx); sessionID != "" {
		newLogger.fields["session_id"] = sessionID
	}

	if traceID := GetTraceID(ctx); traceID != "" {
		newLogger.fields["trace_id"] = traceID
	}

	if userID := GetUserID(ctx); userID != "" {
		newLogger.fields["user_id"] = userID
	}

	return newLogger
}

// WithField adds a field to the logger
func (cl *CorrelationLogger) WithField(key string, value interface{}) *CorrelationLogger {
	newLogger := &CorrelationLogger{
		logger: cl.logger,
		fields: make(map[string]interface{}),
	}

	for k, v := range cl.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adds multiple fields to the logger
func (cl *CorrelationLogger) WithFields(fields map[string]interface{}) *CorrelationLogger {
	newLogger := &CorrelationLogger{
		logger: cl.logger,
		fields: make(map[string]interface{}),
	}

	for k, v := range cl.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// formatMessage formats a log message with correlation fields
func (cl *CorrelationLogger) formatMessage(level, message string) string {
	if len(cl.fields) == 0 {
		return fmt.Sprintf("[%s] %s", level, message)
	}

	fieldStr := ""
	for k, v := range cl.fields {
		if fieldStr != "" {
			fieldStr += " "
		}
		fieldStr += fmt.Sprintf("%s=%v", k, v)
	}

	return fmt.Sprintf("[%s] [%s] %s", level, fieldStr, message)
}

// Info logs an info message
func (cl *CorrelationLogger) Info(message string) {
	cl.logger.Println(cl.formatMessage("INFO", message))
}

// Infof logs a formatted info message
func (cl *CorrelationLogger) Infof(format string, args ...interface{}) {
	cl.Info(fmt.Sprintf(format, args...))
}

// Error logs an error message
func (cl *CorrelationLogger) Error(message string) {
	cl.logger.Println(cl.formatMessage("ERROR", message))
}

// Errorf logs a formatted error message
func (cl *CorrelationLogger) Errorf(format string, args ...interface{}) {
	cl.Error(fmt.Sprintf(format, args...))
}

// Debug logs a debug message
func (cl *CorrelationLogger) Debug(message string) {
	cl.logger.Println(cl.formatMessage("DEBUG", message))
}

// Debugf logs a formatted debug message
func (cl *CorrelationLogger) Debugf(format string, args ...interface{}) {
	cl.Debug(fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func (cl *CorrelationLogger) Warn(message string) {
	cl.logger.Println(cl.formatMessage("WARN", message))
}

// Warnf logs a formatted warning message
func (cl *CorrelationLogger) Warnf(format string, args ...interface{}) {
	cl.Warn(fmt.Sprintf(format, args...))
}

// CorrelationMiddleware adds correlation IDs to HTTP requests
func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get or generate correlation ID
		correlationID := r.Header.Get(CorrelationHeader)
		if correlationID == "" {
			correlationID = GenerateCorrelationID()
		}
		ctx = WithCorrelationID(ctx, correlationID)

		// Get or generate request ID
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = GenerateRequestID()
		}
		ctx = WithRequestID(ctx, requestID)

		// Add correlation IDs to response headers
		w.Header().Set(CorrelationHeader, correlationID)
		w.Header().Set(RequestIDHeader, requestID)

		// Continue with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GenerateCorrelationID generates a new correlation ID
func GenerateCorrelationID() string {
	return "corr-" + uuid.New().String()
}

// GenerateRequestID generates a new request ID
func GenerateRequestID() string {
	return "req-" + uuid.New().String()
}

// GenerateSessionID generates a new session ID
func GenerateSessionID() string {
	return "sess-" + uuid.New().String()
}

// GenerateTraceID generates a new trace ID
func GenerateTraceID() string {
	return "trace-" + uuid.New().String()
}

// WithCorrelationID adds a correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// WithRequestID adds a request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithSessionID adds a session ID to context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// WithTraceID adds a trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithUserID adds a user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetCorrelationID gets correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if val, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return val
	}
	return ""
}

// GetRequestID gets request ID from context
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(RequestIDKey).(string); ok {
		return val
	}
	return ""
}

// GetSessionID gets session ID from context
func GetSessionID(ctx context.Context) string {
	if val, ok := ctx.Value(SessionIDKey).(string); ok {
		return val
	}
	return ""
}

// GetTraceID gets trace ID from context
func GetTraceID(ctx context.Context) string {
	if val, ok := ctx.Value(TraceIDKey).(string); ok {
		return val
	}
	return ""
}

// GetUserID gets user ID from context
func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	TraceID       string                 `json:"trace_id,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	Service       string                 `json:"service,omitempty"`
	Environment   string                 `json:"environment,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Stack         string                 `json:"stack,omitempty"`
}

// NewLogEntry creates a new log entry from context
func NewLogEntry(ctx context.Context, level, message string) *LogEntry {
	entry := &LogEntry{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		CorrelationID: GetCorrelationID(ctx),
		RequestID:     GetRequestID(ctx),
		SessionID:     GetSessionID(ctx),
		TraceID:       GetTraceID(ctx),
		UserID:        GetUserID(ctx),
		Fields:        make(map[string]interface{}),
	}

	return entry
}

// WithField adds a field to the log entry
func (le *LogEntry) WithField(key string, value interface{}) *LogEntry {
	if le.Fields == nil {
		le.Fields = make(map[string]interface{})
	}
	le.Fields[key] = value
	return le
}

// WithError adds error information to the log entry
func (le *LogEntry) WithError(err error) *LogEntry {
	if err != nil {
		le.Error = err.Error()
		// Could add stack trace extraction here
	}
	return le
}
