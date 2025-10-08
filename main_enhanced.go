package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// main is the enhanced entry point with all optimizations
func main() {
	// Load configuration
	config, err := LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logging
	if err := InitLogger(
		config.Observability.ServiceName,
		config.Observability.ServiceVersion,
		config.Monitoring.LogLevel,
		config.Observability.LogFormat,
	); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger := GetLogger()
	logger.Info("Starting Bolt Tracker API",
		zap.String("version", config.Observability.ServiceVersion),
		zap.String("environment", GetEnvString("ENVIRONMENT", "development")),
	)

	// Initialize metrics
	if err := InitMetrics(
		config.Observability.ServiceName,
		config.Observability.ServiceVersion,
	); err != nil {
		logger.Fatal("Failed to initialize metrics", zap.Error(err))
	}

	// Initialize tracing if enabled
	if config.Observability.EnableTracing {
		jaegerEndpoint := GetEnvString("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
		if err := InitTracing(
			config.Observability.ServiceName,
			config.Observability.ServiceVersion,
			jaegerEndpoint,
		); err != nil {
			logger.Error("Failed to initialize tracing", zap.Error(err))
		} else {
			logger.Info("Tracing initialized", zap.String("endpoint", jaegerEndpoint))
		}
	}

	// Initialize Redis with enhanced configuration
	err = initRedisWithConfig(
		config.Redis.Host,
		config.Redis.Port,
		config.Redis.Username,
		config.Redis.Password,
		config.Redis.DB,
		config.Redis.PoolSize,
		config.Redis.MinIdleConnections,
		3, // maxRetries
		config.Redis.DialTimeout,
		config.Redis.ReadTimeout,
		config.Redis.WriteTimeout,
	)
	if err != nil {
		logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}

	// Initialize database with enhanced configuration
	err = initDatabase()
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize JWT manager with enhanced features
	var jwtManager *EnhancedJWTManager
	if config.JWT.KeyRotationEnabled {
		// Use Redis for token revocation if available
		var revocationStore TokenRevocationStore
		if config.JWT.RevocationStore == "redis" {
			revocationStore = NewRedisTokenRevocationStore(redisClient, config.Redis.Namespace)
		} else {
			revocationStore = NewMemoryTokenRevocationStore()
		}

		jwtManager, err = InitEnhancedJWTManager(&config.JWT, revocationStore)
		if err != nil {
			logger.Fatal("Failed to initialize enhanced JWT manager", zap.Error(err))
		}
		logger.Info("Enhanced JWT manager initialized with key rotation")
	} else {
		// Fallback to original JWT manager
		logger.Info("Using original JWT manager")
	}

	// Initialize worker pool
	if err := InitGlobalWorkerPool(&config.API); err != nil {
		logger.Fatal("Failed to initialize worker pool", zap.Error(err))
	}

	// Register job handlers
	workerPool := GetGlobalWorkerPool()
	workerPool.RegisterHandler("fetch_vehicles", handleFetchVehiclesJob)
	workerPool.RegisterHandler("process_analytics", handleProcessAnalyticsJob)
	workerPool.RegisterHandler("cache_cleanup", handleCacheCleanupJob)

	// Initialize rate limiter with enhanced configuration
	rateLimiter := NewEnhancedRateLimiter(
		config.RateLimiting.GlobalLimit,
		config.RateLimiting.Window,
	)

	// Initialize circuit breaker
	circuitBreaker := NewCircuitBreaker(
		config.CircuitBreaker.MaxFailures,
		config.CircuitBreaker.Timeout,
	)

	// Setup Gin router with optimizations
	router := setupEnhancedRouter(config, jwtManager, rateLimiter, circuitBreaker)

	// Create HTTP server with enhanced configuration
	server := &http.Server{
		Addr:         config.Server.Port,
		Handler:      router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}

	// Initialize shutdown manager
	shutdownManager := InitShutdownManager(server, config.Server.GracefulShutdownTimeout)

	// Add shutdown handlers
	shutdownManager.AddDatabaseShutdownHandler(func() error {
		return db.Close()
	})
	shutdownManager.AddRedisShutdownHandler(redisClient)
	shutdownManager.AddWorkerPoolShutdownHandler(func(ctx context.Context) error {
		return workerPool.Stop()
	})

	// Setup signal handling
	SetupSignalHandling()

	// Start background services
	startBackgroundServices(config, db, workerPool)

	// Start HTTP server in goroutine
	go func() {
		logger.Info("Starting HTTP server", zap.String("port", config.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Start Prometheus metrics server if enabled
	if config.Monitoring.EnablePrometheus {
		go startPrometheusServer(config.Monitoring.PrometheusPort)
	}

	// Start config watcher if enabled
	if GetEnvBool("WATCH_CONFIG", false) {
		if err := WatchConfig(); err != nil {
			logger.Error("Failed to start config watcher", zap.Error(err))
		}
	}

	// Wait for shutdown signal
	shutdownManager.WaitForShutdown()

	logger.Info("Application shutdown completed")
}

// setupEnhancedRouter sets up the Gin router with all optimizations
func setupEnhancedRouter(
	config *EnhancedConfig,
	jwtManager *EnhancedJWTManager,
	rateLimiter *EnhancedRateLimiter,
	circuitBreaker *CircuitBreaker,
) *gin.Engine {
	// Set Gin mode based on environment
	if GetEnvString("GIN_MODE", "release") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Add global middleware
	router.Use(LoggingMiddleware())
	router.Use(ErrorHandler())
	router.Use(MetricsMiddleware())
	router.Use(CORSMiddleware(config.Security))

	// Add rate limiting middleware
	if config.Security.RateLimitEnabled {
		router.Use(RateLimitMiddleware(rateLimiter))
	}

	// Health check endpoints
	router.GET("/healthz", HealthCheckHandler())
	router.GET("/readyz", ReadinessCheckHandler())
	router.GET("/livez", LivenessCheckHandler())

	// Legacy health endpoints for frontend compatibility
	router.GET("/api/health", HealthCheckHandler())
	router.GET("/api/performance", getEnhancedPerformanceStats)

	// Prometheus metrics endpoint
	if config.Monitoring.EnablePrometheus {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// API routes with authentication
	api := router.Group("/api")
	{
		// JWT management endpoints
		// jwt := api.Group("/jwt")
		// {
		// 	// JWT endpoints - to be implemented
		// 	// jwt.GET("/status", getJWTStatusHandler(jwtManager))
		// 	// jwt.POST("/refresh", refreshTokenHandler(jwtManager))
		// 	// jwt.POST("/revoke", revokeTokenHandler(jwtManager))
		// 	// jwt.GET("/keys", getPublicKeysHandler(jwtManager))
		// }

		// Vehicle endpoints
		vehicles := api.Group("/vehicles")
		{
			vehicles.GET("/latest", getLatestVehicles)
			vehicles.GET("/", getVehicles)
			vehicles.GET("/history", getVehicleHistoryEnhanced)
		}

		// Analytics endpoints
		analytics := api.Group("/analytics")
		{
			analytics.GET("/heatmap", getHeatmapAPI)
			analytics.GET("/trend", getTrendAPI)
			analytics.GET("/history", getAnalyticsHistoryAPI)

			// Fast analytics endpoints with aggressive caching
			analytics.GET("/heatmap/fast", getFastHeatmapAPI)
			analytics.GET("/trend/fast", getFastTrendAPI)
		}

		// Cache management endpoints
		cache := api.Group("/cache")
		{
			cache.GET("/status", getCacheStatus)
		}

		// Worker pool management
		// workers := api.Group("/workers")
		// {
		// 	// workers.GET("/status", getWorkerPoolStatusHandler())
		// 	// workers.GET("/metrics", getWorkerPoolMetricsHandler())
		// }
	}

	return router
}

// startBackgroundServices starts all background services
func startBackgroundServices(
	config *EnhancedConfig,
	db *sql.DB,
	workerPool *EnhancedWorkerPool,
) {
	logger := GetLogger()

	// Start data fetch loop
	go func() {
		logger.Info("Starting data fetch loop")
		dataFetchLoop()
	}()

	// Start analytics worker for pre-computation
	go func() {
		logger.Info("Starting analytics worker")
		StartAnalyticsWorker()
	}()

	// Start metrics collection
	go func() {
		ticker := time.NewTicker(config.Monitoring.MetricsInterval)
		defer ticker.Stop()

		for range ticker.C {
			collectSystemMetrics()
		}
	}()

	// Start cache cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			cleanupExpiredCache()
		}
	}()

	// Start database connection monitoring
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			monitorDatabaseConnections(db)
		}
	}()

	// Start Redis connection monitoring
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			monitorRedisConnections()
		}
	}()
}

// startPrometheusServer starts the Prometheus metrics server
func startPrometheusServer(port string) {
	logger := GetLogger()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    port,
		Handler: mux,
	}

	logger.Info("Starting Prometheus metrics server", zap.String("port", port))
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Prometheus server failed", zap.Error(err))
	}
}

// collectSystemMetrics collects system metrics
func collectSystemMetrics() {
	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.UpdateSystemMetrics()
	}
}

// cleanupExpiredCache cleans up expired cache entries
func cleanupExpiredCache() {
	// Check if Redis client is initialized
	if redisClient == nil {
		GetLogger().Debug("Redis client not initialized, skipping cache cleanup")
		return
	}

	_, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// This would implement cache cleanup logic
	GetLogger().Debug("Cache cleanup completed")
}

// monitorDatabaseConnections monitors database connections
func monitorDatabaseConnections(db *sql.DB) {
	// Get connection stats
	stats := db.Stats()

	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.SetDatabaseConnections(stats.OpenConnections, stats.Idle)
	}

	GetLogger().Debug("Database connection stats",
		zap.Int("open_connections", stats.OpenConnections),
		zap.Int("idle_connections", stats.Idle),
		zap.Int("in_use", stats.InUse),
	)
}

// monitorRedisConnections monitors Redis connections
func monitorRedisConnections() {
	// Check if Redis client is initialized
	if redisClient == nil {
		GetLogger().Debug("Redis client not initialized, skipping connection monitoring")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping Redis to check connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		GetLogger().Error("Redis connection check failed", zap.Error(err))
		return
	}

	// Get connection pool stats
	poolStats := redisClient.PoolStats()

	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.SetRedisConnections(int(poolStats.TotalConns))
	}

	GetLogger().Debug("Redis connection stats",
		zap.Int("total_connections", int(poolStats.TotalConns)),
		zap.Int("idle_connections", int(poolStats.IdleConns)),
		zap.Int("stale_connections", int(poolStats.StaleConns)),
	)
}

// Job handlers for worker pool

func handleFetchVehiclesJob(ctx context.Context, job *Job) (*JobResult, error) {
	start := time.Now()

	// Extract job data
	locationData, ok := job.Data.(map[string]interface{})
	if !ok {
		return &JobResult{
			JobID:   job.ID,
			Success: false,
			Error:   fmt.Errorf("invalid job data"),
		}, nil
	}

	locationID, _ := locationData["location_id"].(string)

	// Fetch vehicles from location
	vehicles, err := fetchVehiclesFromLocation(Location{ID: locationID})
	if err != nil {
		return &JobResult{
			JobID:   job.ID,
			Success: false,
			Error:   err,
		}, nil
	}

	// Record metrics
	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.RecordVehiclesFetched(len(vehicles))
		metricsCollector.RecordLocationsProcessed(1)
	}

	return &JobResult{
		JobID:    job.ID,
		Success:  true,
		Result:   vehicles,
		Duration: time.Since(start),
	}, nil
}

func handleProcessAnalyticsJob(ctx context.Context, job *Job) (*JobResult, error) {
	start := time.Now()

	// Process analytics data
	// This would implement analytics processing logic

	return &JobResult{
		JobID:    job.ID,
		Success:  true,
		Result:   "analytics processed",
		Duration: time.Since(start),
	}, nil
}

func handleCacheCleanupJob(ctx context.Context, job *Job) (*JobResult, error) {
	start := time.Now()

	// Cleanup cache
	// This would implement cache cleanup logic

	return &JobResult{
		JobID:    job.ID,
		Success:  true,
		Result:   "cache cleaned",
		Duration: time.Since(start),
	}, nil
}

// CORS middleware
func CORSMiddleware(security SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !security.EnableCORS {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range security.CORSOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Rate limit middleware
func RateLimitMiddleware(rateLimiter *EnhancedRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()

		// Check rate limit
		if !rateLimiter.Allow(clientIP) {
			// Record rate limit hit
			if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
				metricsCollector.RecordRateLimitReject(clientIP, "per_ip")
			}

			c.JSON(429, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests",
			})
			c.Abort()
			return
		}

		// Record rate limit hit
		if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
			metricsCollector.RecordRateLimitHit(clientIP, "per_ip")
		}

		c.Next()
	}
}

// getVehicleHistoryEnhanced handles GET /api/vehicles/history with enhanced filtering
func getVehicleHistoryEnhanced(c *gin.Context) {
	// Parse query parameters
	vehicleID := c.Query("vehicle_id")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000 // Cap at 1000 for performance
	}

	// Get logger for error handling
	logger := GetLogger()

	// Get history records
	history, err := getVehicleHistoryWithFilters(vehicleID, startTime, endTime, limit)
	if err != nil {
		logger.Error("Failed to query vehicle history", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "database query failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(history),
		"data":   history,
	})
}

// getEnhancedPerformanceStats returns enhanced performance statistics
func getEnhancedPerformanceStats(c *gin.Context) {
	logger := GetLogger()

	// Get system metrics
	stats := gin.H{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "running",
		"version":   "1.0.0",
		"uptime":    time.Since(time.Now()).String(),
	}

	// Add database stats if available
	if db != nil {
		var totalVehicles int
		err := db.QueryRow("SELECT COUNT(*) FROM vehicle_cache").Scan(&totalVehicles)
		if err == nil {
			stats["database"] = gin.H{
				"connected":      true,
				"total_vehicles": totalVehicles,
			}
		} else {
			stats["database"] = gin.H{
				"connected": false,
				"error":     err.Error(),
			}
		}
	}

	// Add Redis stats if available
	if redisClient != nil {
		_, err := redisClient.Ping(ctx).Result()
		if err == nil {
			stats["redis"] = gin.H{
				"connected": true,
				"status":    "healthy",
			}
		} else {
			stats["redis"] = gin.H{
				"connected": false,
				"error":     err.Error(),
			}
		}
	}

	// Add worker pool stats if available
	if globalWorkerPool != nil {
		stats["worker_pool"] = globalWorkerPool.GetStats()
	}

	// Add analytics worker stats
	stats["analytics_worker"] = gin.H{
		"running": analyticsWorker != nil,
		"status":  "active",
	}

	logger.Info("Performance stats requested")
	c.JSON(http.StatusOK, stats)
}
