package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// EnhancedShutdownManager manages graceful shutdown of the application
type EnhancedShutdownManager struct {
	server           *http.Server
	shutdownTimeout  time.Duration
	shutdownHandlers []ShutdownHandler
	mu               sync.RWMutex
	shutdownChan     chan struct{}
	shutdownOnce     sync.Once
}

// ShutdownHandler represents a function that should be called during shutdown
type ShutdownHandler struct {
	Name     string
	Handler  func(ctx context.Context) error
	Priority int // Lower numbers = higher priority
	Timeout  time.Duration
}

// Global shutdown manager
var (
	shutdownManager  *EnhancedShutdownManager
	shutdownMu       sync.RWMutex
	redisHandlerOnce sync.Once
)

// InitShutdownManager initializes the shutdown manager
func InitShutdownManager(server *http.Server, shutdownTimeout time.Duration) *EnhancedShutdownManager {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()

	if shutdownManager == nil {
		shutdownManager = &EnhancedShutdownManager{
			server:           server,
			shutdownTimeout:  shutdownTimeout,
			shutdownHandlers: make([]ShutdownHandler, 0),
			shutdownChan:     make(chan struct{}),
		}
	}

	return shutdownManager
}

// GetEnhancedShutdownManager returns the global shutdown manager
func GetEnhancedShutdownManager() *EnhancedShutdownManager {
	shutdownMu.RLock()
	defer shutdownMu.RUnlock()
	return shutdownManager
}

// AddShutdownHandler adds a handler to be called during shutdown
func (sm *EnhancedShutdownManager) AddShutdownHandler(handler ShutdownHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.shutdownHandlers = append(sm.shutdownHandlers, handler)
}

// AddDatabaseShutdownHandler adds database shutdown handler
func (sm *EnhancedShutdownManager) AddDatabaseShutdownHandler(dbCloser func() error) {
	sm.AddShutdownHandler(ShutdownHandler{
		Name:     "database",
		Handler:  func(ctx context.Context) error { return dbCloser() },
		Priority: 1,
		Timeout:  10 * time.Second,
	})
}

// AddRedisShutdownHandler adds Redis shutdown handler
func (sm *EnhancedShutdownManager) AddRedisShutdownHandler(client *redis.Client) {
	redisHandlerOnce.Do(func() {
		sm.AddShutdownHandler(ShutdownHandler{
			Name: "redis",
			Handler: func(ctx context.Context) error {
				if client == nil {
					log.Println("Redis client is nil, skipping shutdown")
					return nil
				}
				log.Printf("Closing Redis connection, isNil: %v", client == nil)
				return client.Close()
			},
			Priority: 2,
			Timeout:  5 * time.Second,
		})
	})
}

// AddWorkerPoolShutdownHandler adds worker pool shutdown handler
func (sm *EnhancedShutdownManager) AddWorkerPoolShutdownHandler(workerPoolCloser func(ctx context.Context) error) {
	sm.AddShutdownHandler(ShutdownHandler{
		Name:     "worker_pool",
		Handler:  workerPoolCloser,
		Priority: 3,
		Timeout:  15 * time.Second,
	})
}

// AddMetricsShutdownHandler adds metrics shutdown handler
func (sm *EnhancedShutdownManager) AddMetricsShutdownHandler(metricsCloser func() error) {
	sm.AddShutdownHandler(ShutdownHandler{
		Name:     "metrics",
		Handler:  func(ctx context.Context) error { return metricsCloser() },
		Priority: 4,
		Timeout:  5 * time.Second,
	})
}

// AddTracingShutdownHandler adds tracing shutdown handler
func (sm *EnhancedShutdownManager) AddTracingShutdownHandler(tracingCloser func() error) {
	sm.AddShutdownHandler(ShutdownHandler{
		Name:     "tracing",
		Handler:  func(ctx context.Context) error { return tracingCloser() },
		Priority: 5,
		Timeout:  5 * time.Second,
	})
}

// StartGracefulShutdown starts the graceful shutdown process
func (sm *EnhancedShutdownManager) StartGracefulShutdown() {
	sm.shutdownOnce.Do(func() {
		close(sm.shutdownChan)

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), sm.shutdownTimeout)
		defer cancel()

		// Log shutdown start
		log.Printf("Starting graceful shutdown, timeout: %v, handlers: %d",
			sm.shutdownTimeout, len(sm.shutdownHandlers))

		// Execute shutdown handlers in priority order
		sm.executeShutdownHandlers(ctx)

		// Shutdown HTTP server
		sm.shutdownHTTPServer(ctx)

		log.Println("Graceful shutdown completed")
	})
}

// executeShutdownHandlers executes all shutdown handlers
func (sm *EnhancedShutdownManager) executeShutdownHandlers(ctx context.Context) {
	sm.mu.RLock()
	handlers := make([]ShutdownHandler, len(sm.shutdownHandlers))
	copy(handlers, sm.shutdownHandlers)
	sm.mu.RUnlock()

	// Sort handlers by priority (lower number = higher priority)
	for i := 0; i < len(handlers)-1; i++ {
		for j := i + 1; j < len(handlers); j++ {
			if handlers[i].Priority > handlers[j].Priority {
				handlers[i], handlers[j] = handlers[j], handlers[i]
			}
		}
	}

	// Execute handlers concurrently with individual timeouts
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h ShutdownHandler) {
			defer wg.Done()

			// Create individual timeout context
			handlerCtx, cancel := context.WithTimeout(ctx, h.Timeout)
			defer cancel()

			// Execute handler
			start := time.Now()
			err := h.Handler(handlerCtx)
			duration := time.Since(start)

			if err != nil {
				log.Printf("Shutdown handler failed: %s, error: %v, duration: %v",
					h.Name, err, duration)
			} else {
				log.Printf("Shutdown handler completed: %s, duration: %v",
					h.Name, duration)
			}
		}(handler)
	}

	// Wait for all handlers to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All shutdown handlers completed")
	case <-ctx.Done():
		log.Printf("Shutdown handlers timed out: %v", ctx.Err())
	}
}

// shutdownHTTPServer shuts down the HTTP server
func (sm *EnhancedShutdownManager) shutdownHTTPServer(ctx context.Context) {
	if sm.server == nil {
		return
	}

	log.Println("Shutting down HTTP server")

	// Create a channel to receive the shutdown result
	shutdownResult := make(chan error, 1)

	// Shutdown server in a goroutine
	go func() {
		shutdownResult <- sm.server.Shutdown(ctx)
	}()

	// Wait for shutdown or timeout
	select {
	case err := <-shutdownResult:
		if err != nil {
			log.Printf("HTTP server shutdown failed: %v", err)
		} else {
			log.Println("HTTP server shutdown completed")
		}
	case <-ctx.Done():
		log.Printf("HTTP server shutdown timed out: %v", ctx.Err())
		// Force close if graceful shutdown fails
		sm.server.Close()
	}
}

// WaitForShutdown waits for shutdown signal
func (sm *EnhancedShutdownManager) WaitForShutdown() {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register for interrupt and terminate signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Printf("Received shutdown signal: %s", sig.String())

	// Start graceful shutdown
	sm.StartGracefulShutdown()
}

// IsShuttingDown returns true if shutdown is in progress
func (sm *EnhancedShutdownManager) IsShuttingDown() bool {
	select {
	case <-sm.shutdownChan:
		return true
	default:
		return false
	}
}

// ShutdownChan returns the shutdown channel
func (sm *EnhancedShutdownManager) ShutdownChan() <-chan struct{} {
	return sm.shutdownChan
}

// SetupSignalHandling sets up signal handling for graceful shutdown
func SetupSignalHandling() {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register for interrupt and terminate signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for {
			sig := <-sigChan
			log.Printf("Received signal: %s", sig.String())

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				// Start graceful shutdown
				if sm := GetEnhancedShutdownManager(); sm != nil {
					sm.StartGracefulShutdown()
				}
			case syscall.SIGHUP:
				// Reload configuration
				log.Println("Reloading configuration")
				if err := ReloadConfig(); err != nil {
					log.Printf("Failed to reload configuration: %v", err)
				} else {
					log.Println("Configuration reloaded successfully")
				}
			}
		}
	}()
}

// HealthCheckHandler provides health check endpoint
func HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if shutting down
		if sm := GetEnhancedShutdownManager(); sm != nil && sm.IsShuttingDown() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "shutting_down",
				"message": "Service is shutting down",
			})
			return
		}

		// Basic health check
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// ReadinessCheckHandler provides readiness check endpoint
func ReadinessCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if shutting down
		if sm := GetEnhancedShutdownManager(); sm != nil && sm.IsShuttingDown() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "not_ready",
				"message": "Service is shutting down",
			})
			return
		}

		// Check dependencies
		status := gin.H{
			"status":       "ready",
			"timestamp":    time.Now().Format(time.RFC3339),
			"dependencies": gin.H{},
		}

		// Check database
		if dbStatus := checkDatabaseHealth(); dbStatus != nil {
			status["dependencies"].(gin.H)["database"] = dbStatus
		}

		// Check Redis
		if redisStatus := checkRedisHealth(); redisStatus != nil {
			status["dependencies"].(gin.H)["redis"] = redisStatus
		}

		// Check if all dependencies are healthy
		deps := status["dependencies"].(gin.H)
		allHealthy := true
		for _, dep := range deps {
			if depStatus, ok := dep.(gin.H); ok {
				if depStatus["status"] != "healthy" {
					allHealthy = false
					break
				}
			}
		}

		if !allHealthy {
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}

		c.JSON(http.StatusOK, status)
	}
}

// LivenessCheckHandler provides liveness check endpoint
func LivenessCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple liveness check
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// checkDatabaseHealth checks database health
func checkDatabaseHealth() gin.H {
	// This would be implemented based on your database connection
	// For now, return a placeholder
	return gin.H{
		"status":  "healthy",
		"message": "Database connection is healthy",
	}
}

// checkRedisHealth checks Redis health
func checkRedisHealth() gin.H {
	// This would be implemented based on your Redis connection
	// For now, return a placeholder
	return gin.H{
		"status":  "healthy",
		"message": "Redis connection is healthy",
	}
}
