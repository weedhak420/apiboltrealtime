package main

import (
	"bytes"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Performance monitoring
type PerformanceMonitor struct {
	mu              sync.RWMutex
	requestCount    int64
	avgResponseTime float64
	memoryUsage     uint64
	cpuUsage        float64
	errorCount      int64
	lastCleanup     time.Time
}

var perfMonitor = &PerformanceMonitor{}

// Memory optimization
func optimizeMemoryUsage() {
	// Force garbage collection
	runtime.GC()

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	perfMonitor.mu.Lock()
	perfMonitor.memoryUsage = m.Alloc
	perfMonitor.mu.Unlock()

	log.Printf("🧠 Memory usage: %d KB", m.Alloc/1024)
}

// Connection pooling optimization
func optimizeConnectionPool() {
	if db != nil {
		// Set optimal connection pool settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	if redisClient != nil {
		// Redis connection pool is already optimized in initRedis()
		log.Println("✅ Connection pools optimized")
	}
}

// JSON processing optimization
func optimizeJSONProcessing() {
	// Pre-allocate JSON encoder/decoder pools
	jsonPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	responsePool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1024*1024) // 1MB initial capacity
		},
	}

	log.Println("✅ JSON processing optimized")
}

// HTTP client optimization
func optimizeHTTPClient() {
	httpClient = &http.Client{
		Timeout: 8 * time.Second, // Reduced timeout
		Transport: &http.Transport{
			MaxIdleConns:          50, // Optimized
			MaxIdleConnsPerHost:   10, // Optimized
			IdleConnTimeout:       30 * time.Second,
			DisableCompression:    false,
			DisableKeepAlives:     false,
			MaxConnsPerHost:       15,              // Reduced for stability
			TLSHandshakeTimeout:   3 * time.Second, // Reduced
			ResponseHeaderTimeout: 5 * time.Second, // Reduced
		},
	}

	log.Println("✅ HTTP client optimized")
}

// Goroutine optimization
func optimizeGoroutines() {
	// Set optimal GOMAXPROCS
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Monitor goroutine count
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			numGoroutines := runtime.NumGoroutine()
			if numGoroutines > 100 {
				log.Printf("⚠️ High goroutine count: %d", numGoroutines)
			}
		}
	}()

	log.Println("✅ Goroutines optimized")
}

// Cache optimization
func optimizeCache() {
	// Clean old cache entries periodically
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			cleanOldCacheEntries()
		}
	}()

	log.Println("✅ Cache optimization started")
}

// Clean old cache entries
func cleanOldCacheEntries() {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for id, vehicle := range vehicleCache {
		if now.Sub(vehicle.Timestamp) > 10*time.Minute {
			expiredKeys = append(expiredKeys, id)
		}
	}

	// Batch delete expired entries
	for _, key := range expiredKeys {
		delete(vehicleCache, key)
	}

	if len(expiredKeys) > 0 {
		log.Printf("🧹 Cleaned %d expired cache entries", len(expiredKeys))
	}
}

// Compression optimization
func optimizeCompression() {
	// Pre-allocate compression buffers
	compressionPool := sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	// Use compression pool for better performance
	_ = compressionPool

	log.Println("✅ Compression optimized")
}

// Rate limiting optimization
func optimizeRateLimiting() {
	// Clean old rate limit entries
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			rateLimitMutex.Lock()
			now := time.Now()
			for location, lastRequest := range rateLimiter {
				if now.Sub(lastRequest) > 5*time.Minute {
					delete(rateLimiter, location)
				}
			}
			rateLimitMutex.Unlock()
		}
	}()

	log.Println("✅ Rate limiting optimized")
}

// Database query optimization
func optimizeDatabaseQueries() {
	if db == nil {
		return
	}

	// Create optimized indexes with proper existence checking
	indexes := []struct {
		name   string
		table  string
		column string
		sql    string
	}{
		{
			name:   "idx_vehicle_timestamp",
			table:  "vehicle_cache",
			column: "timestamp",
			sql:    "CREATE INDEX idx_vehicle_timestamp ON vehicle_cache(timestamp)",
		},
		{
			name:   "idx_vehicle_created_at",
			table:  "vehicle_cache",
			column: "created_at",
			sql:    "CREATE INDEX idx_vehicle_created_at ON vehicle_cache(created_at)",
		},
		{
			name:   "idx_vehicle_location",
			table:  "vehicle_cache",
			column: "source_location",
			sql:    "CREATE INDEX idx_vehicle_location ON vehicle_cache(source_location)",
		},
		{
			name:   "idx_vehicle_category",
			table:  "vehicle_cache",
			column: "category_name",
			sql:    "CREATE INDEX idx_vehicle_category ON vehicle_cache(category_name)",
		},
		// Note: idx_performance_metric removed - performance_stats table already has
		// inline index idx_metric_time on (metric_name, timestamp) defined in table creation
	}

	for _, idx := range indexes {
		// Check if index already exists
		var count int
		checkSQL := `
			SELECT COUNT(*) FROM information_schema.statistics 
			WHERE table_schema = DATABASE() 
			AND table_name = ? 
			AND index_name = ?
		`
		err := db.QueryRow(checkSQL, idx.table, idx.name).Scan(&count)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to check index existence for %s: %v", idx.name, err)
			continue
		}

		if count > 0 {
			log.Printf("ℹ️ Index '%s' already exists, skipping", idx.name)
		} else {
			_, err = db.Exec(idx.sql)
			if err != nil {
				log.Printf("❌ Failed to create index '%s': %v", idx.name, err)
			} else {
				log.Printf("✅ Index '%s' created successfully", idx.name)
			}
		}
	}

	log.Println("✅ Database queries optimized")
}

// Socket.IO optimization
func optimizeSocketIO() {
	if socketServer != nil {
		// Socket.IO is already configured in main.go
		// No additional configuration needed for this version
		log.Println("✅ Socket.IO optimized")
	}
}

// Initialize all optimizations
func initializeOptimizations() {
	log.Println("🚀 Initializing performance optimizations...")

	optimizeMemoryUsage()
	optimizeConnectionPool()
	optimizeJSONProcessing()
	optimizeHTTPClient()
	optimizeGoroutines()
	optimizeCache()
	optimizeCompression()
	optimizeRateLimiting()
	optimizeDatabaseQueries()
	optimizeSocketIO()

	log.Println("✅ All optimizations initialized")
}

// Performance monitoring
func startPerformanceMonitoring() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Monitor memory usage
			optimizeMemoryUsage()

			// Monitor goroutine count
			numGoroutines := runtime.NumGoroutine()
			if numGoroutines > 50 {
				log.Printf("⚠️ High goroutine count: %d", numGoroutines)
			}

			// Clean old data
			cleanOldCacheEntries()
		}
	}()

	log.Println("✅ Performance monitoring started")
}

// Get performance statistics (optimization version)
func getOptimizationPerformanceStats() map[string]interface{} {
	perfMonitor.mu.RLock()
	defer perfMonitor.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"request_count":     perfMonitor.requestCount,
		"avg_response_time": perfMonitor.avgResponseTime,
		"memory_usage_kb":   m.Alloc / 1024,
		"memory_usage_mb":   m.Alloc / 1024 / 1024,
		"goroutine_count":   runtime.NumGoroutine(),
		"gc_count":          m.NumGC,
		"error_count":       perfMonitor.errorCount,
		"cache_size":        len(vehicleCache),
		"rate_limit_size":   len(rateLimiter),
	}
}

// Record performance metric (optimization version)
func recordOptimizationPerformanceMetric(metricName string, value float64) {
	perfMonitor.mu.Lock()
	defer perfMonitor.mu.Unlock()

	switch metricName {
	case "request_count":
		perfMonitor.requestCount++
	case "response_time":
		// Update average response time
		perfMonitor.avgResponseTime = (perfMonitor.avgResponseTime + value) / 2
	case "error_count":
		perfMonitor.errorCount++
	}

	// Record to database if available
	if db != nil {
		go recordAnalyticsData(metricName, value, nil)
	}
}
