package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogLevel represents logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelFatal LogLevel = "fatal"
	LevelPanic LogLevel = "panic"
)

// Logger wraps zerolog with additional functionality
type Logger struct {
	logger  zerolog.Logger
	service string
	fields  map[string]interface{}
}

// Config holds logger configuration
type Config struct {
	Level       LogLevel
	Service     string
	Environment string
	Output      io.Writer
	PrettyLog   bool
	AddCaller   bool
}

// DefaultConfig returns default logger configuration
func DefaultConfig(service string) *Config {
	return &Config{
		Level:       LevelInfo,
		Service:     service,
		Environment: getEnv("ENVIRONMENT", "development"),
		Output:      os.Stdout,
		PrettyLog:   getEnv("ENVIRONMENT", "development") == "development",
		AddCaller:   true,
	}
}

// NewLogger creates a new structured logger
func NewLogger(config *Config) *Logger {
	if config == nil {
		config = DefaultConfig("unknown")
	}

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Set log level
	level := parseLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	// Configure output
	var output io.Writer = config.Output
	if output == nil {
		output = os.Stdout
	}

	if config.PrettyLog {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "15:04:05.000",
			NoColor:    false,
		}
	}

	// Create logger
	logger := zerolog.New(output).
		With().
		Timestamp().
		Str("service", config.Service).
		Str("environment", config.Environment).
		Str("version", getEnv("SERVICE_VERSION", "unknown")).
		Logger()

	if config.AddCaller {
		logger = logger.With().Caller().Logger()
	}

	return &Logger{
		logger:  logger,
		service: config.Service,
		fields:  make(map[string]interface{}),
	}
}

// WithContext creates a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newLogger := l.logger.With().Logger()

	// Extract common context values
	if requestID := extractRequestID(ctx); requestID != "" {
		newLogger = newLogger.With().Str("request_id", requestID).Logger()
	}
	if userID := extractUserID(ctx); userID != "" {
		newLogger = newLogger.With().Str("user_id", userID).Logger()
	}
	if sessionID := extractSessionID(ctx); sessionID != "" {
		newLogger = newLogger.With().Str("session_id", sessionID).Logger()
	}
	if traceID := extractTraceID(ctx); traceID != "" {
		newLogger = newLogger.With().Str("trace_id", traceID).Logger()
	}

	return &Logger{
		logger:  newLogger,
		service: l.service,
		fields:  l.fields,
	}
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger:  l.logger.With().Interface(key, value).Logger(),
		service: l.service,
		fields:  l.fields,
	}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := l.logger.With().Fields(fields).Logger()
	return &Logger{
		logger:  newLogger,
		service: l.service,
		fields:  fields,
	}
}

// WithError adds an error to the logger
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}

	newLogger := l.logger.With().
		Err(err).
		Str("error_type", fmt.Sprintf("%T", err)).
		Logger()

	// Add stack trace for errors
	if stack := getStackTrace(2); len(stack) > 0 {
		newLogger = newLogger.With().Strs("stack", stack).Logger()
	}

	return &Logger{
		logger:  newLogger,
		service: l.service,
		fields:  l.fields,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logger.Debug().Msgf(format, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logger.Info().Msgf(format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logger.Warn().Msgf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.logger.Error().Msg(msg)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logger.Error().Msgf(format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.logger.Fatal().Msg(msg)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal().Msgf(format, args...)
}

// Panic logs a panic message and panics
func (l *Logger) Panic(msg string) {
	l.logger.Panic().Msg(msg)
}

// Panicf logs a formatted panic message and panics
func (l *Logger) Panicf(format string, args ...interface{}) {
	l.logger.Panic().Msgf(format, args...)
}

// Audit logs an audit event
func (l *Logger) Audit(event string, fields map[string]interface{}) {
	auditLogger := l.logger.With().
		Str("audit_event", event).
		Time("audit_timestamp", time.Now()).
		Fields(fields).
		Logger()

	auditLogger.Info().Msg("AUDIT")
}

// Performance logs a performance metric
func (l *Logger) Performance(operation string, duration time.Duration, fields map[string]interface{}) {
	perfLogger := l.logger.With().
		Str("operation", operation).
		Dur("duration_ms", duration).
		Fields(fields).
		Logger()

	// Log as warning if operation is slow
	if duration > 1*time.Second {
		perfLogger.Warn().Msg("SLOW_OPERATION")
	} else {
		perfLogger.Info().Msg("PERFORMANCE")
	}
}

// Security logs a security event
func (l *Logger) Security(event string, severity string, fields map[string]interface{}) {
	secLogger := l.logger.With().
		Str("security_event", event).
		Str("severity", severity).
		Time("security_timestamp", time.Now()).
		Fields(fields).
		Logger()

	switch severity {
	case "critical", "high":
		secLogger.Error().Msg("SECURITY")
	case "medium":
		secLogger.Warn().Msg("SECURITY")
	default:
		secLogger.Info().Msg("SECURITY")
	}
}

// Helper functions

func parseLevel(level LogLevel) zerolog.Level {
	switch level {
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	case LevelFatal:
		return zerolog.FatalLevel
	case LevelPanic:
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getStackTrace(skip int) []string {
	var stack []string
	for i := skip; i < skip+5; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
		}
	}
	return stack
}

// Context extractors (implement based on your context structure)

func extractRequestID(ctx context.Context) string {
	if v := ctx.Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractUserID(ctx context.Context) string {
	if v := ctx.Value("user_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractSessionID(ctx context.Context) string {
	if v := ctx.Value("session_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractTraceID(ctx context.Context) string {
	if v := ctx.Value("trace_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger
func Init(config *Config) {
	globalLogger = NewLogger(config)
}

// Default returns the default global logger
func Default() *Logger {
	if globalLogger == nil {
		Init(DefaultConfig("default"))
	}
	return globalLogger
}

// Helper functions for migration from log/fmt.Printf

// ReplaceStandardLog replaces standard log output
func ReplaceStandardLog() {
	log.Logger = globalLogger.logger
}

// MigrationHelper helps migrate from fmt.Printf/log.Printf
type MigrationHelper struct {
	logger *Logger
}

// NewMigrationHelper creates a migration helper
func NewMigrationHelper(serviceName string) *MigrationHelper {
	return &MigrationHelper{
		logger: NewLogger(DefaultConfig(serviceName)),
	}
}

// Printf replaces fmt.Printf with structured logging
func (m *MigrationHelper) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Try to detect log level from message
	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed"):
		m.logger.Error(msg)
	case strings.Contains(msgLower, "warning") || strings.Contains(msgLower, "warn"):
		m.logger.Warn(msg)
	case strings.Contains(msgLower, "debug"):
		m.logger.Debug(msg)
	default:
		m.logger.Info(msg)
	}
}

// LogPrintf replaces log.Printf with structured logging
func (m *MigrationHelper) LogPrintf(format string, args ...interface{}) {
	m.Printf(format, args...)
}
