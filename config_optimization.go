package main

import (
	"log"
	"runtime"
	"time"
)

// Optimization configuration
type OptimizationConfig struct {
	// Memory settings
	MaxMemoryMB          int           `json:"max_memory_mb"`
	GCInterval           time.Duration `json:"gc_interval"`
	CacheCleanupInterval time.Duration `json:"cache_cleanup_interval"`

	// Database settings
	MaxDBConns      int           `json:"max_db_conns"`
	MaxIdleConnsDB  int           `json:"max_idle_conns_db"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`

	// Redis settings
	RedisPoolSize     int `json:"redis_pool_size"`
	RedisMinIdleConns int `json:"redis_min_idle_conns"`
	RedisMaxRetries   int `json:"redis_max_retries"`

	// HTTP settings
	HTTPTimeout      time.Duration `json:"http_timeout"`
	MaxIdleConnsHTTP int           `json:"max_idle_conns_http"`
	MaxConnsPerHost  int           `json:"max_conns_per_host"`

	// Goroutine settings
	MaxGoroutines          int           `json:"max_goroutines"`
	GoroutineCheckInterval time.Duration `json:"goroutine_check_interval"`

	// Cache settings
	CacheExpiry  time.Duration `json:"cache_expiry"`
	MaxCacheSize int           `json:"max_cache_size"`

	// Rate limiting
	RateLimitInterval    time.Duration `json:"rate_limit_interval"`
	MaxRequestsPerMinute int           `json:"max_requests_per_minute"`
}

// Default optimization configuration
var defaultOptimizationConfig = OptimizationConfig{
	// Memory settings
	MaxMemoryMB:          512, // 512MB max memory
	GCInterval:           30 * time.Second,
	CacheCleanupInterval: 5 * time.Minute,

	// Database settings
	MaxDBConns:      25,
	MaxIdleConnsDB:  5,
	ConnMaxLifetime: 5 * time.Minute,

	// Redis settings
	RedisPoolSize:     10,
	RedisMinIdleConns: 5,
	RedisMaxRetries:   3,

	// HTTP settings
	HTTPTimeout:      8 * time.Second,
	MaxIdleConnsHTTP: 50,
	MaxConnsPerHost:  15,

	// Goroutine settings
	MaxGoroutines:          100,
	GoroutineCheckInterval: 30 * time.Second,

	// Cache settings
	CacheExpiry:  10 * time.Minute,
	MaxCacheSize: 1000,

	// Rate limiting
	RateLimitInterval:    1 * time.Second,
	MaxRequestsPerMinute: 30,
}

// Current optimization configuration
var optimizationConfig = defaultOptimizationConfig

// Apply optimization configuration
func applyOptimizationConfig(config OptimizationConfig) {
	optimizationConfig = config

	// Apply memory settings
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Apply database settings
	if db != nil {
		db.SetMaxOpenConns(config.MaxDBConns)
		db.SetMaxIdleConns(config.MaxIdleConnsDB)
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	// Apply Redis settings
	if redisClient != nil {
		// Redis client settings are applied in initRedis()
	}

	log.Printf("✅ Optimization configuration applied")
}

// Get current optimization configuration
func getOptimizationConfig() OptimizationConfig {
	return optimizationConfig
}

// Update optimization configuration
func updateOptimizationConfig(updates map[string]interface{}) {
	// Update specific settings based on the updates map
	// This is a simplified version - you might want to implement proper validation

	if maxMemory, ok := updates["max_memory_mb"].(int); ok {
		optimizationConfig.MaxMemoryMB = maxMemory
	}

	if maxDBConns, ok := updates["max_db_conns"].(int); ok {
		optimizationConfig.MaxDBConns = maxDBConns
		if db != nil {
			db.SetMaxOpenConns(maxDBConns)
		}
	}

	if maxGoroutines, ok := updates["max_goroutines"].(int); ok {
		optimizationConfig.MaxGoroutines = maxGoroutines
	}

	log.Printf("✅ Optimization configuration updated")
}

// Monitor system resources
func monitorSystemResources() {
	go func() {
		ticker := time.NewTicker(optimizationConfig.GoroutineCheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			// Check memory usage
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			memoryMB := m.Alloc / 1024 / 1024
			if memoryMB > uint64(optimizationConfig.MaxMemoryMB) {
				log.Printf("⚠️ High memory usage: %d MB (max: %d MB)", memoryMB, optimizationConfig.MaxMemoryMB)
				runtime.GC() // Force garbage collection
			}

			// Check goroutine count
			numGoroutines := runtime.NumGoroutine()
			if numGoroutines > optimizationConfig.MaxGoroutines {
				log.Printf("⚠️ High goroutine count: %d (max: %d)", numGoroutines, optimizationConfig.MaxGoroutines)
			}

			// Check cache size
			cacheMu.RLock()
			cacheSize := len(vehicleCache)
			cacheMu.RUnlock()

			if cacheSize > optimizationConfig.MaxCacheSize {
				log.Printf("⚠️ Large cache size: %d (max: %d)", cacheSize, optimizationConfig.MaxCacheSize)
				cleanOldCacheEntries()
			}
		}
	}()

	log.Println("✅ System resource monitoring started")
}

// Get system resource statistics
func getSystemResourceStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cacheMu.RLock()
	cacheSize := len(vehicleCache)
	cacheMu.RUnlock()

	rateLimitMutex.RLock()
	rateLimitSize := len(rateLimiter)
	rateLimitMutex.RUnlock()

	return map[string]interface{}{
		"memory_usage_mb": m.Alloc / 1024 / 1024,
		"memory_sys_mb":   m.Sys / 1024 / 1024,
		"goroutine_count": runtime.NumGoroutine(),
		"gc_count":        m.NumGC,
		"cache_size":      cacheSize,
		"rate_limit_size": rateLimitSize,
		"max_memory_mb":   optimizationConfig.MaxMemoryMB,
		"max_goroutines":  optimizationConfig.MaxGoroutines,
		"max_cache_size":  optimizationConfig.MaxCacheSize,
	}
}

// Initialize optimization monitoring
func initializeOptimizationMonitoring() {
	// Start system resource monitoring
	monitorSystemResources()

	// Start periodic cleanup
	go func() {
		ticker := time.NewTicker(optimizationConfig.CacheCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			cleanOldCacheEntries()
			optimizeMemoryUsage()
		}
	}()

	log.Println("✅ Optimization monitoring initialized")
}
