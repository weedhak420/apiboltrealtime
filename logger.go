package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with additional functionality
type Logger struct {
	*zap.Logger
	serviceName    string
	serviceVersion string
}

// LogFields represents structured log fields
type LogFields map[string]interface{}

// CorrelationIDKey is the context key for correlation ID
type CorrelationIDKey struct{}

// Global logger instance
var (
	globalLogger *Logger
	loggerMu     sync.RWMutex
)

// InitLogger initializes the global logger
func InitLogger(serviceName, serviceVersion, logLevel, logFormat string) error {
	var config zap.Config

	if logFormat == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Set log level
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(level)

	// Custom encoder config
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Add service information
	config.InitialFields = map[string]interface{}{
		"service":  serviceName,
		"version":  serviceVersion,
		"hostname": getHostname(),
		"pid":      os.Getpid(),
	}

	// Build logger
	zapLogger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	// Wrap with our custom logger
	globalLogger = &Logger{
		Logger:         zapLogger,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
	}

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return globalLogger
}

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey{}, correlationID)
}

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(CorrelationIDKey{}).(string); ok {
		return correlationID
	}
	return ""
}

// GenerateCorrelationID generates a new correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// WithFields creates a logger with additional fields
func (l *Logger) WithFields(fields LogFields) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}

	return &Logger{
		Logger:         l.Logger.With(zapFields...),
		serviceName:    l.serviceName,
		serviceVersion: l.serviceVersion,
	}
}

// WithContext creates a logger with context information
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := LogFields{}

	// Add correlation ID if present
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		fields["correlation_id"] = correlationID
	}

	// Add request ID if present
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields["request_id"] = requestID
	}

	// Add user ID if present
	if userID := ctx.Value("user_id"); userID != nil {
		fields["user_id"] = userID
	}

	return l.WithFields(fields)
}

// WithError creates a logger with error information
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger:         l.Logger.With(zap.Error(err)),
		serviceName:    l.serviceName,
		serviceVersion: l.serviceVersion,
	}
}

// WithStack creates a logger with stack trace
func (l *Logger) WithStack() *Logger {
	stack := make([]byte, 1024)
	length := runtime.Stack(stack, false)
	return &Logger{
		Logger:         l.Logger.With(zap.String("stack", string(stack[:length]))),
		serviceName:    l.serviceName,
		serviceVersion: l.serviceVersion,
	}
}

// LogRequest logs HTTP request information
func (l *Logger) LogRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, fields LogFields) {
	requestFields := LogFields{
		"method":      method,
		"path":        path,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		requestFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(requestFields)

	if statusCode >= 400 {
		logger.Error("HTTP request completed with error")
	} else {
		logger.Info("HTTP request completed")
	}
}

// LogDatabaseOperation logs database operation
func (l *Logger) LogDatabaseOperation(ctx context.Context, operation string, table string, duration time.Duration, err error, fields LogFields) {
	dbFields := LogFields{
		"operation":   operation,
		"table":       table,
		"duration_ms": duration.Milliseconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		dbFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(dbFields)

	if err != nil {
		logger.WithError(err).Error("Database operation failed")
	} else {
		logger.Info("Database operation completed")
	}
}

// LogCacheOperation logs cache operation
func (l *Logger) LogCacheOperation(ctx context.Context, operation string, key string, hit bool, duration time.Duration, err error, fields LogFields) {
	cacheFields := LogFields{
		"operation":   operation,
		"key":         key,
		"hit":         hit,
		"duration_ms": duration.Milliseconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		cacheFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(cacheFields)

	if err != nil {
		logger.WithError(err).Error("Cache operation failed")
	} else {
		logger.Info("Cache operation completed")
	}
}

// LogAPICall logs external API call
func (l *Logger) LogAPICall(ctx context.Context, service string, endpoint string, method string, statusCode int, duration time.Duration, err error, fields LogFields) {
	apiFields := LogFields{
		"service":     service,
		"endpoint":    endpoint,
		"method":      method,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		apiFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(apiFields)

	if err != nil {
		logger.WithError(err).Error("API call failed")
	} else if statusCode >= 400 {
		logger.Error("API call completed with error")
	} else {
		logger.Info("API call completed")
	}
}

// LogWorkerOperation logs worker pool operation
func (l *Logger) LogWorkerOperation(ctx context.Context, operation string, workerID string, jobID string, duration time.Duration, err error, fields LogFields) {
	workerFields := LogFields{
		"operation":   operation,
		"worker_id":   workerID,
		"job_id":      jobID,
		"duration_ms": duration.Milliseconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		workerFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(workerFields)

	if err != nil {
		logger.WithError(err).Error("Worker operation failed")
	} else {
		logger.Info("Worker operation completed")
	}
}

// LogRateLimit logs rate limiting event
func (l *Logger) LogRateLimit(ctx context.Context, key string, limit int, window time.Duration, fields LogFields) {
	rateLimitFields := LogFields{
		"rate_limit_key": key,
		"limit":          limit,
		"window_seconds": window.Seconds(),
	}

	// Merge additional fields
	for key, value := range fields {
		rateLimitFields[key] = value
	}

	l.WithContext(ctx).WithFields(rateLimitFields).Warn("Rate limit exceeded")
}

// LogCircuitBreaker logs circuit breaker state change
func (l *Logger) LogCircuitBreaker(ctx context.Context, service string, state string, failureCount int, fields LogFields) {
	cbFields := LogFields{
		"service":       service,
		"state":         state,
		"failure_count": failureCount,
	}

	// Merge additional fields
	for key, value := range fields {
		cbFields[key] = value
	}

	l.WithContext(ctx).WithFields(cbFields).Info("Circuit breaker state changed")
}

// LogSecurity logs security-related events
func (l *Logger) LogSecurity(ctx context.Context, event string, severity string, fields LogFields) {
	securityFields := LogFields{
		"event":    event,
		"severity": severity,
	}

	// Merge additional fields
	for key, value := range fields {
		securityFields[key] = value
	}

	logger := l.WithContext(ctx).WithFields(securityFields)

	switch severity {
	case "critical":
		logger.Error("Security event: " + event)
	case "high":
		logger.Warn("Security event: " + event)
	default:
		logger.Info("Security event: " + event)
	}
}

// LogPerformance logs performance metrics
func (l *Logger) LogPerformance(ctx context.Context, metric string, value float64, unit string, fields LogFields) {
	perfFields := LogFields{
		"metric": metric,
		"value":  value,
		"unit":   unit,
	}

	// Merge additional fields
	for key, value := range fields {
		perfFields[key] = value
	}

	l.WithContext(ctx).WithFields(perfFields).Info("Performance metric")
}

// getHostname returns the hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// LoggingMiddleware creates a Gin middleware for request logging
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Generate correlation ID if not present
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = GenerateCorrelationID()
		}

		// Add to context
		ctx := WithCorrelationID(c.Request.Context(), correlationID)
		c.Request = c.Request.WithContext(ctx)

		// Set correlation ID in response header
		c.Header("X-Correlation-ID", correlationID)

		// Process request
		c.Next()

		// Log request
		duration := time.Since(start)
		fields := LogFields{
			"user_agent":  c.GetHeader("User-Agent"),
			"remote_addr": c.ClientIP(),
		}

		GetLogger().LogRequest(ctx, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration, fields)
	}
}

// ErrorHandler handles panics and errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				GetLogger().WithContext(c.Request.Context()).
					WithFields(LogFields{
						"panic": err,
						"path":  c.Request.URL.Path,
					}).
					WithStack().
					Error("Panic recovered")

				// Return error response
				c.JSON(500, gin.H{
					"error":   "Internal server error",
					"message": "An unexpected error occurred",
				})
			}
		}()

		c.Next()
	}
}
