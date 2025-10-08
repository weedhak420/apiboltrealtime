package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// MetricsCollector handles all application metrics
type MetricsCollector struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Database metrics
	DatabaseOperationsTotal   *prometheus.CounterVec
	DatabaseOperationDuration *prometheus.HistogramVec
	DatabaseConnectionsActive prometheus.Gauge
	DatabaseConnectionsIdle   prometheus.Gauge

	// Redis metrics
	RedisOperationsTotal   *prometheus.CounterVec
	RedisOperationDuration *prometheus.HistogramVec
	RedisConnectionsActive prometheus.Gauge
	RedisCacheHits         *prometheus.CounterVec
	RedisCacheMisses       *prometheus.CounterVec

	// Worker pool metrics
	WorkerPoolJobsTotal     *prometheus.CounterVec
	WorkerPoolJobDuration   *prometheus.HistogramVec
	WorkerPoolActiveWorkers prometheus.Gauge
	WorkerPoolQueuedJobs    prometheus.Gauge

	// Rate limiting metrics
	RateLimitHits    *prometheus.CounterVec
	RateLimitRejects *prometheus.CounterVec

	// Circuit breaker metrics
	CircuitBreakerStateChanges *prometheus.CounterVec
	CircuitBreakerFailures     *prometheus.CounterVec

	// Business metrics
	VehiclesFetchedTotal    prometheus.Counter
	VehiclesProcessedTotal  prometheus.Counter
	LocationsProcessedTotal prometheus.Counter
	APIErrorsTotal          *prometheus.CounterVec

	// System metrics
	GoroutinesCount prometheus.Gauge
	MemoryUsage     prometheus.Gauge
	CPUUsage        prometheus.Gauge
}

// Global metrics collector
var (
	metricsCollector *MetricsCollector
	tracer           oteltrace.Tracer
)

// InitMetrics initializes the metrics collector
func InitMetrics(serviceName, serviceVersion string) error {
	metricsCollector = &MetricsCollector{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),

		// Database metrics
		DatabaseOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "database_operations_total",
				Help: "Total number of database operations",
			},
			[]string{"operation", "table", "status"},
		),
		DatabaseOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "database_operation_duration_seconds",
				Help:    "Database operation duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"operation", "table"},
		),
		DatabaseConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "database_connections_active",
				Help: "Number of active database connections",
			},
		),
		DatabaseConnectionsIdle: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "database_connections_idle",
				Help: "Number of idle database connections",
			},
		),

		// Redis metrics
		RedisOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_operations_total",
				Help: "Total number of Redis operations",
			},
			[]string{"operation", "status"},
		),
		RedisOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "redis_operation_duration_seconds",
				Help:    "Redis operation duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"operation"},
		),
		RedisConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "redis_connections_active",
				Help: "Number of active Redis connections",
			},
		),
		RedisCacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_cache_hits_total",
				Help: "Total number of Redis cache hits",
			},
			[]string{"key_pattern"},
		),
		RedisCacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_cache_misses_total",
				Help: "Total number of Redis cache misses",
			},
			[]string{"key_pattern"},
		),

		// Worker pool metrics
		WorkerPoolJobsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "worker_pool_jobs_total",
				Help: "Total number of worker pool jobs",
			},
			[]string{"worker_id", "status"},
		),
		WorkerPoolJobDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "worker_pool_job_duration_seconds",
				Help:    "Worker pool job duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"worker_id"},
		),
		WorkerPoolActiveWorkers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "worker_pool_active_workers",
				Help: "Number of active workers",
			},
		),
		WorkerPoolQueuedJobs: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "worker_pool_queued_jobs",
				Help: "Number of queued jobs",
			},
		),

		// Rate limiting metrics
		RateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"key", "limit_type"},
		),
		RateLimitRejects: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_rejects_total",
				Help: "Total number of rate limit rejects",
			},
			[]string{"key", "limit_type"},
		),

		// Circuit breaker metrics
		CircuitBreakerStateChanges: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_state_changes_total",
				Help: "Total number of circuit breaker state changes",
			},
			[]string{"service", "state"},
		),
		CircuitBreakerFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_failures_total",
				Help: "Total number of circuit breaker failures",
			},
			[]string{"service"},
		),

		// Business metrics
		VehiclesFetchedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "vehicles_fetched_total",
				Help: "Total number of vehicles fetched",
			},
		),
		VehiclesProcessedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "vehicles_processed_total",
				Help: "Total number of vehicles processed",
			},
		),
		LocationsProcessedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "locations_processed_total",
				Help: "Total number of locations processed",
			},
		),
		APIErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_errors_total",
				Help: "Total number of API errors",
			},
			[]string{"service", "error_type"},
		),

		// System metrics
		GoroutinesCount: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "goroutines_count",
				Help: "Number of goroutines",
			},
		),
		MemoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Memory usage in bytes",
			},
		),
		CPUUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cpu_usage_percent",
				Help: "CPU usage percentage",
			},
		),
	}

	return nil
}

// GetMetricsCollector returns the global metrics collector
func GetMetricsCollector() *MetricsCollector {
	return metricsCollector
}

// RecordHTTPRequest records HTTP request metrics
func (mc *MetricsCollector) RecordHTTPRequest(method, path, statusCode string, duration time.Duration) {
	mc.HTTPRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
	mc.HTTPRequestDuration.WithLabelValues(method, path, statusCode).Observe(duration.Seconds())
}

// RecordDatabaseOperation records database operation metrics
func (mc *MetricsCollector) RecordDatabaseOperation(operation, table, status string, duration time.Duration) {
	mc.DatabaseOperationsTotal.WithLabelValues(operation, table, status).Inc()
	mc.DatabaseOperationDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordRedisOperation records Redis operation metrics
func (mc *MetricsCollector) RecordRedisOperation(operation, status string, duration time.Duration) {
	mc.RedisOperationsTotal.WithLabelValues(operation, status).Inc()
	mc.RedisOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordRedisCacheHit records Redis cache hit
func (mc *MetricsCollector) RecordRedisCacheHit(keyPattern string) {
	mc.RedisCacheHits.WithLabelValues(keyPattern).Inc()
}

// RecordRedisCacheMiss records Redis cache miss
func (mc *MetricsCollector) RecordRedisCacheMiss(keyPattern string) {
	mc.RedisCacheMisses.WithLabelValues(keyPattern).Inc()
}

// RecordWorkerPoolJob records worker pool job metrics
func (mc *MetricsCollector) RecordWorkerPoolJob(workerID, status string, duration time.Duration) {
	mc.WorkerPoolJobsTotal.WithLabelValues(workerID, status).Inc()
	mc.WorkerPoolJobDuration.WithLabelValues(workerID).Observe(duration.Seconds())
}

// RecordRateLimitHit records rate limit hit
func (mc *MetricsCollector) RecordRateLimitHit(key, limitType string) {
	mc.RateLimitHits.WithLabelValues(key, limitType).Inc()
}

// RecordRateLimitReject records rate limit reject
func (mc *MetricsCollector) RecordRateLimitReject(key, limitType string) {
	mc.RateLimitRejects.WithLabelValues(key, limitType).Inc()
}

// RecordCircuitBreakerStateChange records circuit breaker state change
func (mc *MetricsCollector) RecordCircuitBreakerStateChange(service, state string) {
	mc.CircuitBreakerStateChanges.WithLabelValues(service, state).Inc()
}

// RecordCircuitBreakerFailure records circuit breaker failure
func (mc *MetricsCollector) RecordCircuitBreakerFailure(service string) {
	mc.CircuitBreakerFailures.WithLabelValues(service).Inc()
}

// RecordVehiclesFetched records vehicles fetched
func (mc *MetricsCollector) RecordVehiclesFetched(count int) {
	mc.VehiclesFetchedTotal.Add(float64(count))
}

// RecordVehiclesProcessed records vehicles processed
func (mc *MetricsCollector) RecordVehiclesProcessed(count int) {
	mc.VehiclesProcessedTotal.Add(float64(count))
}

// RecordLocationsProcessed records locations processed
func (mc *MetricsCollector) RecordLocationsProcessed(count int) {
	mc.LocationsProcessedTotal.Add(float64(count))
}

// RecordAPIError records API error
func (mc *MetricsCollector) RecordAPIError(service, errorType string) {
	mc.APIErrorsTotal.WithLabelValues(service, errorType).Inc()
}

// UpdateSystemMetrics updates system metrics
func (mc *MetricsCollector) UpdateSystemMetrics() {
	// Update goroutines count
	mc.GoroutinesCount.Set(float64(runtime.NumGoroutine()))

	// Update memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	mc.MemoryUsage.Set(float64(m.Alloc))

	// CPU usage would need external monitoring
	// This is a placeholder
	mc.CPUUsage.Set(0.0)
}

// SetDatabaseConnections sets database connection metrics
func (mc *MetricsCollector) SetDatabaseConnections(active, idle int) {
	mc.DatabaseConnectionsActive.Set(float64(active))
	mc.DatabaseConnectionsIdle.Set(float64(idle))
}

// SetRedisConnections sets Redis connection metrics
func (mc *MetricsCollector) SetRedisConnections(active int) {
	mc.RedisConnectionsActive.Set(float64(active))
}

// SetWorkerPoolMetrics sets worker pool metrics
func (mc *MetricsCollector) SetWorkerPoolMetrics(activeWorkers, queuedJobs int) {
	mc.WorkerPoolActiveWorkers.Set(float64(activeWorkers))
	mc.WorkerPoolQueuedJobs.Set(float64(queuedJobs))
}

// SetHTTPRequestsInFlight sets HTTP requests in flight
func (mc *MetricsCollector) SetHTTPRequestsInFlight(count int) {
	mc.HTTPRequestsInFlight.Set(float64(count))
}

// InitTracing initializes OpenTelemetry tracing
func InitTracing(serviceName, serviceVersion, jaegerEndpoint string) error {
	// Create Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
	if err != nil {
		return err
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return err
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Create tracer
	tracer = otel.Tracer(serviceName)

	return nil
}

// GetTracer returns the global tracer
func GetTracer() oteltrace.Tracer {
	return tracer
}

// StartSpan starts a new span
func StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, name, opts...)
}

// AddSpanAttributes adds attributes to a span
func AddSpanAttributes(span oteltrace.Span, attrs map[string]interface{}) {
	for key, value := range attrs {
		span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
	}
}

// MetricsMiddleware creates a Gin middleware for metrics collection
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Increment requests in flight
		GetMetricsCollector().HTTPRequestsInFlight.Inc()
		defer GetMetricsCollector().HTTPRequestsInFlight.Dec()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		GetMetricsCollector().RecordHTTPRequest(
			c.Request.Method,
			c.Request.URL.Path,
			fmt.Sprintf("%d", c.Writer.Status()),
			duration,
		)
	}
}
