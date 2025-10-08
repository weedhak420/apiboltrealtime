package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status    string                   `json:"status"`
	Timestamp time.Time                `json:"timestamp"`
	Services  map[string]ServiceHealth `json:"services"`
	Metrics   SystemMetrics            `json:"metrics"`
	Uptime    time.Duration            `json:"uptime"`
}

// ServiceHealth represents the health of an individual service
type ServiceHealth struct {
	Status    string        `json:"status"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	LastCheck time.Time     `json:"last_check"`
}

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	Goroutines    int     `json:"goroutines"`
	MemoryUsage   int64   `json:"memory_usage_bytes"`
	CacheSize     int     `json:"cache_size"`
	RateLimitSize int     `json:"rate_limit_size"`
	CPUUsage      float64 `json:"cpu_usage_percent"`
}

// HealthMonitor manages health checking for all services
type HealthMonitor struct {
	startTime     time.Time
	checkInterval time.Duration
	stopChan      chan bool
	mutex         sync.RWMutex
	lastCheck     time.Time
	services      map[string]ServiceHealth
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(checkInterval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		startTime:     time.Now(),
		checkInterval: checkInterval,
		stopChan:      make(chan bool),
		services:      make(map[string]ServiceHealth),
	}
}

// Start starts the health monitoring
func (hm *HealthMonitor) Start() {
	log.Println("🏥 Starting health monitoring...")

	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.performHealthCheck()
		case <-hm.stopChan:
			log.Println("🛑 Health monitoring stopped")
			return
		}
	}
}

// Stop stops the health monitoring
func (hm *HealthMonitor) Stop() {
	close(hm.stopChan)
}

// performHealthCheck performs a comprehensive health check
func (hm *HealthMonitor) performHealthCheck() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	now := time.Now()

	// Check database
	hm.services["database"] = hm.checkDatabase()

	// Check Redis
	hm.services["redis"] = hm.checkRedis()

	// Check API
	hm.services["api"] = hm.checkAPI()

	// Check worker pool
	hm.services["worker_pool"] = hm.checkWorkerPool()

	// Check circuit breakers
	hm.services["circuit_breakers"] = hm.checkCircuitBreakers()

	hm.lastCheck = now
}

// checkDatabase checks database health
func (hm *HealthMonitor) checkDatabase() ServiceHealth {
	start := time.Now()

	if db == nil {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   0,
			Error:     "database not initialized",
			LastCheck: time.Now(),
		}
	}

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.PingContext(ctx)
	latency := time.Since(start)

	if err != nil {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   latency,
			Error:     err.Error(),
			LastCheck: time.Now(),
		}
	}

	return ServiceHealth{
		Status:    "healthy",
		Latency:   latency,
		LastCheck: time.Now(),
	}
}

// checkRedis checks Redis health
func (hm *HealthMonitor) checkRedis() ServiceHealth {
	start := time.Now()

	if redisClient == nil {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   0,
			Error:     "Redis not initialized",
			LastCheck: time.Now(),
		}
	}

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := redisClient.Ping(ctx).Err()
	latency := time.Since(start)

	if err != nil {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   latency,
			Error:     err.Error(),
			LastCheck: time.Now(),
		}
	}

	return ServiceHealth{
		Status:    "healthy",
		Latency:   latency,
		LastCheck: time.Now(),
	}
}

// checkAPI checks API health
func (hm *HealthMonitor) checkAPI() ServiceHealth {
	start := time.Now()

	// Check if API circuit breaker is open
	if apiCircuitBreaker != nil && apiCircuitBreaker.GetState() == StateOpen {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   time.Since(start),
			Error:     "API circuit breaker is open",
			LastCheck: time.Now(),
		}
	}

	return ServiceHealth{
		Status:    "healthy",
		Latency:   time.Since(start),
		LastCheck: time.Now(),
	}
}

// checkWorkerPool checks worker pool health
func (hm *HealthMonitor) checkWorkerPool() ServiceHealth {
	start := time.Now()

	if globalWorkerPool == nil {
		return ServiceHealth{
			Status:    "unhealthy",
			Latency:   0,
			Error:     "Worker pool not initialized",
			LastCheck: time.Now(),
		}
	}

	stats := globalWorkerPool.GetStats()
	queueSize := stats["queue_size"].(int)
	queueCap := stats["queue_cap"].(int)

	// Check if queue is too full
	if queueSize > queueCap*80/100 { // 80% full
		return ServiceHealth{
			Status:    "degraded",
			Latency:   time.Since(start),
			Error:     fmt.Sprintf("Worker pool queue is %d%% full", queueSize*100/queueCap),
			LastCheck: time.Now(),
		}
	}

	return ServiceHealth{
		Status:    "healthy",
		Latency:   time.Since(start),
		LastCheck: time.Now(),
	}
}

// checkCircuitBreakers checks circuit breaker health
func (hm *HealthMonitor) checkCircuitBreakers() ServiceHealth {
	start := time.Now()

	openBreakers := 0
	totalBreakers := 0

	if apiCircuitBreaker != nil {
		totalBreakers++
		if apiCircuitBreaker.GetState() == StateOpen {
			openBreakers++
		}
	}

	if databaseCircuitBreaker != nil {
		totalBreakers++
		if databaseCircuitBreaker.GetState() == StateOpen {
			openBreakers++
		}
	}

	if redisCircuitBreaker != nil {
		totalBreakers++
		if redisCircuitBreaker.GetState() == StateOpen {
			openBreakers++
		}
	}

	status := "healthy"
	if openBreakers > 0 {
		status = "degraded"
		if openBreakers == totalBreakers {
			status = "unhealthy"
		}
	}

	return ServiceHealth{
		Status:    status,
		Latency:   time.Since(start),
		LastCheck: time.Now(),
	}
}

// GetHealthStatus returns the current health status
func (hm *HealthMonitor) GetHealthStatus() HealthStatus {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	// Get system metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get cache size
	cacheMu.RLock()
	cacheSize := len(vehicleCache)
	cacheMu.RUnlock()

	// Get rate limiter size
	rateLimitMutex.RLock()
	rateLimitSize := len(rateLimiter)
	rateLimitMutex.RUnlock()

	metrics := SystemMetrics{
		Goroutines:    runtime.NumGoroutine(),
		MemoryUsage:   int64(memStats.Alloc),
		CacheSize:     cacheSize,
		RateLimitSize: rateLimitSize,
		CPUUsage:      0.0, // Would need external library for CPU usage
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, service := range hm.services {
		if service.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if service.Status == "degraded" && overallStatus == "healthy" {
			overallStatus = "degraded"
		}
	}

	return HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Services:  hm.services,
		Metrics:   metrics,
		Uptime:    time.Since(hm.startTime),
	}
}

// Global health monitor
var globalHealthMonitor *HealthMonitor

// InitializeHealthMonitor initializes the global health monitor
func InitializeHealthMonitor() {
	globalHealthMonitor = NewHealthMonitor(30 * time.Second) // Check every 30 seconds
	go globalHealthMonitor.Start()
}

// GetHealthStatus returns the current health status
func GetHealthStatus() HealthStatus {
	if globalHealthMonitor == nil {
		return HealthStatus{
			Status:    "unhealthy",
			Timestamp: time.Now(),
			Services:  make(map[string]ServiceHealth),
			Metrics:   SystemMetrics{},
			Uptime:    0,
		}
	}
	return globalHealthMonitor.GetHealthStatus()
}
