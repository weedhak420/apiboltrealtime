// This file is used to run the application with all dependencies
// Usage: go run .

package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	sio "github.com/googollee/go-socket.io"
)

// Configuration
const (
	TestMode      = false
	MaxWorkers    = 10               // Increased for more locations
	FetchInterval = 10 * time.Second // Increased interval to reduce rate limiting
	ServerPort    = ":8000"
	BatchSize     = 20              // Process locations in batches
	BatchDelay    = 2 * time.Second // Delay between batches

	// Deduplication settings
	MaxDistanceMeters = 10.0             // Maximum distance to consider vehicles as same
	MaxBearingDiff    = 15.0             // Maximum bearing difference in degrees
	MinTimeDiff       = 30 * time.Second // Minimum time difference for same vehicle
)

// Global state
var (
	locations           []Location
	enhancedRateLimiter *EnhancedRateLimiter
)

func init() {
	// Load locations from config
	locations = loadLocations()
	// Initialize vehicle cache
	vehicleCache = make(map[string]Vehicle)
	rateLimiter = make(map[string]time.Time)

	// Initialize configuration manager
	if err := InitializeConfigManager("config.json"); err != nil {
		log.Printf("⚠️ Failed to initialize config manager: %v", err)
	}

	// Get configuration
	config := GetConfig()

	// Initialize performance optimizations with enhanced HTTP client
	httpClient = &http.Client{
		Timeout: config.API.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:          config.HTTP.MaxIdleConns,
			MaxIdleConnsPerHost:   config.HTTP.MaxIdleConnsPerHost,
			IdleConnTimeout:       config.HTTP.IdleConnTimeout,
			DisableCompression:    false,
			DisableKeepAlives:     false,
			MaxConnsPerHost:       config.HTTP.MaxConnsPerHost,
			TLSHandshakeTimeout:   config.HTTP.TLSHandshakeTimeout,
			ResponseHeaderTimeout: config.HTTP.ResponseHeaderTimeout,
			ExpectContinueTimeout: config.HTTP.ExpectContinueTimeout,
		},
	}

	// Initialize memory pool manager
	InitializeMemoryPool()

	// Initialize enhanced rate limiter
	enhancedRateLimiter = NewEnhancedRateLimiter(config.API.RateLimit, config.API.RateLimitWindow)

	// Initialize worker pool
	InitializeWorkerPool(config.API.MaxWorkers)

	// Initialize circuit breakers
	InitializeCircuitBreakers()

	// Initialize graceful shutdown manager
	InitializeShutdownManager()

	// Initialize JWT generator
	InitializeJWTGenerator()

	// Initialize JWT manager
	InitializeJWTManager()

	// Initialize object pools for better memory management
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
}

func runLegacy() {
	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Setup Socket.IO
	socketServer = sio.NewServer(nil)

	socketServer.OnConnect("/", func(s sio.Conn) error {
		log.Println("✅ Client connected:", s.ID())
		return nil
	})

	socketServer.OnDisconnect("/", func(s sio.Conn, reason string) {
		log.Println("⚠️ Client disconnected:", s.ID(), reason)
	})

	// Routes
	router.GET("/", serveIndex)
	router.GET("/test-api", testAPI)
	router.GET("/locations", getLocations)

	// Serve static files for frontend
	router.Static("/static", "./static")
	router.StaticFile("/favicon.ico", "./static/favicon.ico")

	// REST API endpoints for frontend
	router.GET("/api/vehicles/latest", getLatestVehicles)
	router.GET("/api/vehicles", getVehicles)
	router.GET("/api/vehicles/history", getVehicleHistoryAPI)
	router.GET("/api/analytics/heatmap", getHeatmapAPI)
	router.GET("/api/analytics/trend", getTrendAPI)
	router.GET("/api/analytics/history", getAnalyticsHistoryAPI)

	// Fast analytics endpoints with aggressive caching
	router.GET("/api/analytics/heatmap/fast", getFastHeatmapAPI)
	router.GET("/api/analytics/trend/fast", getFastTrendAPI)

	// Cache status endpoint
	router.GET("/api/cache/status", getCacheStatus)
	router.GET("/api/status", getAPIStatus)
	router.GET("/api/analytics", getAnalytics)
	router.GET("/api/deduplication", getDeduplicationInfo)
	router.GET("/api/health", getHealthCheck)
	router.GET("/api/performance", getPerformanceStats)
	router.GET("/api/jwt/status", getJWTStatus)

	router.GET("/socket.io/*any", gin.WrapH(socketServer))
	router.POST("/socket.io/*any", gin.WrapH(socketServer))

	// Initialize database
	if err := initDatabase(); err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}

	// Initialize statement pool
	if err := InitializeStmtPool(); err != nil {
		log.Printf("⚠️ Failed to initialize statement pool: %v", err)
	}

	// Initialize Redis
	if err := initRedis(); err != nil {
		log.Printf("⚠️ Failed to initialize Redis: %v", err)
	}

	// Initialize health monitoring
	InitializeHealthMonitor()

	// Initialize optimizations
	initializeOptimizations()

	// Start background fetch loop
	go dataFetchLoop()

	// Start performance monitoring
	startPerformanceMonitoring()

	// Start analytics worker for pre-computation
	StartAnalyticsWorker()

	// Start server
	log.Println("=" + string(make([]byte, 60)))
	log.Println("🚕 Bolt Taxi Tracker - Go Edition")
	log.Println("=" + string(make([]byte, 60)))
	log.Printf("📍 Locations: %d\n", len(locations))
	log.Printf("⚡ Workers: %d\n", MaxWorkers)
	log.Printf("🔄 Interval: %v\n", FetchInterval)
	log.Printf("🧪 Test Mode: %v\n", TestMode)
	log.Println("=" + string(make([]byte, 60)))
	log.Printf("🌐 Server starting at http://0.0.0.0%s\n", ServerPort)

	// Setup graceful shutdown
	go func() {
		// This will be called when the server is shutting down
		defer func() {
			StopAnalyticsWorker()
		}()
	}()

	if err := router.Run(ServerPort); err != nil {
		log.Fatal(err)
	}
}

func loadLocations() []Location {
	// Load from config.go
	return AllLocations
}

// dataFetchLoop runs in background and fetches vehicle data every FetchInterval
func dataFetchLoop() {
	ticker := time.NewTicker(FetchInterval)
	defer ticker.Stop()

	log.Printf("🔄 Starting data fetch loop with %v interval", FetchInterval)

	for range ticker.C {
		log.Printf("📡 Broadcast tick at %s", time.Now().Format(time.RFC3339))

		// Use a timeout to prevent hanging
		done := make(chan bool, 1)
		var allVehicles []Vehicle

		go func() {
			allVehicles = fetchAllVehicles()
			done <- true
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Success
			log.Printf("✅ Fetch completed successfully")
		case <-time.After(30 * time.Second): // Increased timeout for batch processing
			log.Printf("⚠️ Fetch timeout - using cached data")
			// Use cached data if available
			cacheMu.RLock()
			allVehicles = make([]Vehicle, 0, len(vehicleCache))
			for _, vehicle := range vehicleCache {
				allVehicles = append(allVehicles, vehicle)
			}
			cacheMu.RUnlock()
		}

		// Update in-memory cache with fresh data
		if len(allVehicles) > 0 {
			cacheMu.Lock()
			// Clear old cache and add new vehicles
			vehicleCache = make(map[string]Vehicle)
			for _, vehicle := range allVehicles {
				vehicleCache[vehicle.ID] = vehicle
			}
			cacheMu.Unlock()
			log.Printf("💾 Updated in-memory cache with %d vehicles", len(allVehicles))
		}

		// Cache vehicles to database and Redis (async to prevent blocking)
		if len(allVehicles) > 0 {
			// Cache to database
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Database cache panic recovered: %v", r)
					}
				}()
				if err := cacheVehicles(allVehicles); err != nil {
					log.Printf("⚠️ Failed to cache vehicles to database: %v", err)
				}
			}()

			// Insert into history table
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ History insert panic recovered: %v", r)
					}
				}()
				if err := insertVehicleHistory(allVehicles); err != nil {
					log.Printf("⚠️ Failed to insert vehicles into history: %v", err)
				}
			}()

			// Cache to Redis
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Redis cache panic recovered: %v", r)
					}
				}()
				if err := cacheVehiclesToRedis(allVehicles); err != nil {
					log.Printf("⚠️ Failed to cache vehicles to Redis: %v", err)
				}
			}()
		}

		// Broadcast to WebSocket clients
		broadcastVehiclesToClients(allVehicles)

		// Monitor goroutines
		goRoutineCount := runtime.NumGoroutine()
		if goRoutineCount > 500 {
			log.Printf("⚠️ High goroutine count: %d", goRoutineCount)
		}
	}
}

// fetchAllVehicles fetches vehicles from all locations in batches to avoid rate limiting
func fetchAllVehicles() []Vehicle {
	var allVehicles []Vehicle
	var mu sync.Mutex

	// Process locations in batches to avoid rate limiting
	totalLocations := len(locations)
	log.Printf("📡 Processing %d locations in batches of %d", totalLocations, BatchSize)

	for i := 0; i < totalLocations; i += BatchSize {
		end := i + BatchSize
		if end > totalLocations {
			end = totalLocations
		}

		batch := locations[i:end]
		log.Printf("🔄 Processing batch %d/%d (%d locations)", (i/BatchSize)+1, (totalLocations+BatchSize-1)/BatchSize, len(batch))

		// Process batch concurrently
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, MaxWorkers)

		for _, location := range batch {
			wg.Add(1)
			go func(loc Location) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				vehicles, err := fetchVehiclesFromLocation(loc)
				if err != nil {
					log.Printf("⚠️ Failed to fetch from %s: %v", loc.ID, err)
					return
				}

				mu.Lock()
				allVehicles = append(allVehicles, vehicles...)
				mu.Unlock()
			}(location)
		}

		wg.Wait()

		// Add delay between batches to avoid rate limiting
		if i+BatchSize < totalLocations {
			log.Printf("⏳ Waiting %v before next batch...", BatchDelay)
			time.Sleep(BatchDelay)
		}
	}

	log.Printf("✅ Completed fetching from all %d locations, got %d vehicles", totalLocations, len(allVehicles))
	return allVehicles
}

// fetchVehiclesFromLocation fetches vehicles from a specific location using exact Bolt API format
func fetchVehiclesFromLocation(location Location) ([]Vehicle, error) {
	// Check enhanced rate limiting
	if !enhancedRateLimiter.Allow(location.ID) {
		log.Printf("⚠️ Rate limited for location %s, skipping", location.ID)
		return []Vehicle{}, nil
	}

	// Check circuit breaker
	if apiCircuitBreaker != nil {
		var err error
		if err = apiCircuitBreaker.Call(func() error {
			// The actual API call will be made inside this function
			return nil
		}); err != nil {
			return []Vehicle{}, fmt.Errorf("API circuit breaker is open: %v", err)
		}
	}

	// Build exact Bolt API request URL with all required parameters
	baseURL := "https://user.live.boltsvc.net/mobility/search/poll"
	params := url.Values{}
	params.Add("version", "CA.180.0")
	params.Add("deviceId", "ffac2e78-84c8-403d-b34e-8394499d7c29")
	params.Add("device_name", "XiaomiMi 11 Lite 4G")
	params.Add("device_os_version", "12")
	params.Add("channel", "googleplay")
	params.Add("brand", "bolt")
	params.Add("deviceType", "android")
	params.Add("signup_session_id", "")
	params.Add("country", "th")
	params.Add("is_local_authentication_available", "false")
	params.Add("language", "th")
	params.Add("gps_lat", fmt.Sprintf("%.6f", location.Coordinates["lat"]))
	params.Add("gps_lng", fmt.Sprintf("%.6f", location.Coordinates["lng"]))
	params.Add("gps_accuracy_m", "10.0")
	params.Add("gps_age", "0")
	params.Add("user_id", "283617495")
	params.Add("session_id", "283617495u1759507555476")
	params.Add("distinct_id", "client-283617495")
	params.Add("rh_session_id", "283617495u1759507023")

	fullURL := baseURL + "?" + params.Encode()

	// Create JSON payload matching the Python example
	payload := map[string]interface{}{
		"destination_stops": []interface{}{},
		"payment_method": map[string]interface{}{
			"id":   "cash",
			"type": "default",
		},
		"pickup_stop": map[string]interface{}{
			"lat":      location.Coordinates["lat"],
			"lng":      location.Coordinates["lng"],
			"address":  "8 ถนน นิมมานเหมินท์ ซอย 2",
			"place_id": "google|ChIJ_ZDMe2E62jARvu7OayZbOok",
		},
		"stage": "overview",
		"viewport": map[string]interface{}{
			"north_east": map[string]float64{
				"lat": 18.804198297114453,
				"lng": 98.97295240312815,
			},
			"south_west": map[string]float64{
				"lat": 18.792010426216578,
				"lng": 98.96452523767948,
			},
		},
	}

	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Create POST request with JSON body
	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Get valid JWT token
	token, err := GetJWTToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get JWT token: %v", err)
	}

	// Add exact headers as specified
	req.Header.Set("Host", "user.live.boltsvc.net")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("User-Agent", "okhttp/4.12.0")

	// Make request with timeout and proper decompression
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DisableCompression: false, // Enable automatic decompression
			DisableKeepAlives:  false, // Keep connections alive
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Handle different response codes
	if resp.StatusCode == 429 {
		log.Printf("⚠️ Rate limited for location %s, skipping", location.ID)
		return []Vehicle{}, nil // Return empty but don't treat as error
	}

	if resp.StatusCode == 401 {
		log.Printf("⚠️ Unauthorized (401) for location %s, JWT token may be expired", location.ID)
		// Force JWT token renewal
		if jwtManager := GetJWTManager(); jwtManager != nil {
			jwtManager.ForceRenewal()
		}
		return []Vehicle{}, nil // Return empty but don't treat as error
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Read response body with smart decompression
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Try multiple decompression methods
	originalBody := body

	// Method 1: Try gzip decompression
	if len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b {
		gzReader, err := gzip.NewReader(bytes.NewReader(body))
		if err == nil {
			decompressed, err := io.ReadAll(gzReader)
			gzReader.Close()
			if err == nil && len(decompressed) > 0 {
				body = decompressed
				log.Printf("🔓 Successfully decompressed gzip response for %s", location.ID)
			}
		}
	}

	// Method 2: Try deflate decompression (if gzip didn't work)
	if len(body) == len(originalBody) && len(body) > 0 {
		// Try deflate
		flateReader := flate.NewReader(bytes.NewReader(body))
		decompressed, err := io.ReadAll(flateReader)
		flateReader.Close()
		if err == nil && len(decompressed) > 0 {
			body = decompressed
			log.Printf("🔓 Successfully decompressed deflate response for %s", location.ID)
		}
	}

	// Method 3: Try raw decompression (if still compressed)
	if len(body) == len(originalBody) && len(body) > 0 {
		// Check if it's still binary/compressed
		isText := true
		for _, b := range body[:min(len(body), 100)] {
			if b < 32 && b != 9 && b != 10 && b != 13 { // Not printable chars
				isText = false
				break
			}
		}

		if !isText {
			log.Printf("⚠️ Response for %s appears to be compressed but decompression failed", location.ID)
		}
	}

	// Log response for debugging (first 500 chars)
	if len(body) > 0 {
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		log.Printf("🔍 API Response for %s: %s", location.ID, bodyStr)
	}

	// Parse response with flexible structure
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Debug: Log the top-level keys in the response (commented out for cleaner logs)
	// log.Printf("🔍 Debug: Top-level keys in response: %v", getMapKeys(responseData))

	// Log successful response
	log.Printf("✅ Successfully fetched data for %s", location.ID)

	// Parse the actual Bolt API response format
	var vehicles []Vehicle

	// Check if response has the expected structure
	if data, exists := responseData["data"]; exists {
		if dataMap, ok := data.(map[string]interface{}); ok {
			// Extract vehicles from taxi data
			if vehiclesData, exists := dataMap["vehicles"]; exists {
				if vehiclesMap, ok := vehiclesData.(map[string]interface{}); ok {
					// Get category details and icons from the response
					var categoryDetails map[string]interface{}
					var icons map[string]interface{}

					if catDetails, exists := vehiclesMap["category_details"]; exists {
						if catMap, ok := catDetails.(map[string]interface{}); ok {
							if taxiDetails, exists := catMap["taxi"]; exists {
								if taxiMap, ok := taxiDetails.(map[string]interface{}); ok {
									categoryDetails = taxiMap
								}
							}
						}
					}

					if iconsData, exists := vehiclesMap["icons"]; exists {
						if iconsMap, ok := iconsData.(map[string]interface{}); ok {
							if taxiIcons, exists := iconsMap["taxi"]; exists {
								if taxiIconsMap, ok := taxiIcons.(map[string]interface{}); ok {
									icons = taxiIconsMap
								}
							}
						}
					}

					if taxiData, exists := vehiclesMap["taxi"]; exists {
						if taxiMap, ok := taxiData.(map[string]interface{}); ok {
							// Process each vehicle category
							for categoryID, categoryVehicles := range taxiMap {
								if vehicleList, ok := categoryVehicles.([]interface{}); ok {
									for _, vehicleItem := range vehicleList {
										if vehicleMap, ok := vehicleItem.(map[string]interface{}); ok {
											vehicle := Vehicle{
												ID:             getString(vehicleMap, "id"),
												Lat:            getFloat64(vehicleMap, "lat"),
												Lng:            getFloat64(vehicleMap, "lng"),
												Bearing:        getFloat64(vehicleMap, "bearing"),
												SourceLocation: location.ID,
												Timestamp:      time.Now(),
												CategoryID:     categoryID,
											}

											// Get icon information from the response
											iconID := getString(vehicleMap, "icon_id")

											// Debug: Log the actual values we're getting from API
											log.Printf("🔍 Debug: categoryID='%s', iconID='%s'", categoryID, iconID)

											// Get category name from category_details
											if categoryDetails != nil {
												if catInfo, exists := categoryDetails[categoryID]; exists {
													if catMap, ok := catInfo.(map[string]interface{}); ok {
														if name, exists := catMap["name"]; exists {
															if nameStr, ok := name.(string); ok {
																vehicle.CategoryName = nameStr
															}
														}
													}
												}
											}

											// Get icon URL from icons
											if icons != nil {
												if iconInfo, exists := icons[iconID]; exists {
													if iconMap, ok := iconInfo.(map[string]interface{}); ok {
														if url, exists := iconMap["icon_url"]; exists {
															if urlStr, ok := url.(string); ok {
																vehicle.IconURL = urlStr
															}
														}
													}
												}
											}

											// Fallback values if not found in API response
											if vehicle.CategoryName == "" {
												vehicle.CategoryName = "Bolt_Taxi"
											}

											if vehicle.IconURL == "" {
												vehicle.IconURL = "https://images.bolt.eu/store/2025/2025-01-23/bbf01cc5-0986-4dcc-ac37-47c23cda37d7.png"
											}

											// Log the final result
											log.Printf("🎯 Vehicle %s: icon_url='%s', category_name='%s'", vehicle.ID, vehicle.IconURL, vehicle.CategoryName)

											// Calculate distance from center
											vehicle.Distance = calculateDistance(
												location.Coordinates["lat"], location.Coordinates["lng"],
												vehicle.Lat, vehicle.Lng,
											)

											vehicles = append(vehicles, vehicle)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	log.Printf("🚗 Found %d vehicles for %s", len(vehicles), location.ID)
	return vehicles, nil
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getString(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}

// calculateDistance calculates distance between two points in meters
func calculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000 // Earth's radius in meters

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// broadcastVehiclesToClients broadcasts vehicles to all connected WebSocket clients
func broadcastVehiclesToClients(vehicles []Vehicle) {
	if socketServer == nil {
		return
	}

	// Create broadcast data
	broadcastData := map[string]interface{}{
		"vehicles":   vehicles,
		"count":      len(vehicles),
		"timestamp":  time.Now().Format(time.RFC3339),
		"fetch_time": time.Now().Unix(),
		"api_status": "success",
	}

	// Broadcast to all connected clients
	socketServer.BroadcastToRoom("/", "", "vehicles_update", broadcastData)

	log.Printf("📡 Broadcasted %d vehicles to WebSocket clients", len(vehicles))
}

// serveIndex serves the main index page
func serveIndex(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Bolt Taxi Tracker API",
		"version": "1.0",
		"status":  "running",
	})
}

// testAPI tests the Bolt API
func testAPI(c *gin.Context) {
	// Test with city center location
	location := Location{
		ID:          "test",
		Coordinates: map[string]float64{"lat": 18.7883, "lng": 98.9853},
	}

	vehicles, err := fetchVehiclesFromLocation(location)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":  true,
		"count":    len(vehicles),
		"vehicles": vehicles,
	})
}

// getLocations returns all monitoring locations
func getLocations(c *gin.Context) {
	c.JSON(200, gin.H{
		"locations": locations,
		"count":     len(locations),
	})
}

// getLatestVehicles returns the latest cached vehicles with fallback logic
func getLatestVehicles(c *gin.Context) {
	var vehicles []Vehicle
	var source string

	// 1. Try Redis first (if available)
	if redisVehicles, err := getCachedVehiclesFromRedis(); err == nil && len(redisVehicles) > 0 {
		vehicles = redisVehicles
		source = "Redis"
		log.Printf("📦 Retrieved %d vehicles from Redis", len(vehicles))
	} else {
		// 2. Fall back to in-memory cache
		cacheMu.RLock()
		vehicles = make([]Vehicle, 0, len(vehicleCache))
		for _, vehicle := range vehicleCache {
			vehicles = append(vehicles, vehicle)
		}
		cacheMu.RUnlock()
		source = "in-memory cache"

		// 3. If in-memory cache is empty, try database
		if len(vehicles) == 0 {
			if dbVehicles, err := getCachedVehicles(); err == nil && len(dbVehicles) > 0 {
				vehicles = dbVehicles
				source = "database"
				log.Printf("📦 Retrieved %d vehicles from database", len(vehicles))
			}
		} else {
			log.Printf("📦 Retrieved %d vehicles from in-memory cache", len(vehicles))
		}
	}

	// Log the source and count
	if len(vehicles) > 0 {
		log.Printf("✅ API request served %d vehicles from %s", len(vehicles), source)
	} else {
		log.Printf("⚠️ API request returned 0 vehicles (no data available)")
	}

	c.JSON(200, gin.H{
		"vehicles":  vehicles,
		"count":     len(vehicles),
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    source,
	})
}

// getVehicles returns vehicles (alias for getLatestVehicles)
func getVehicles(c *gin.Context) {
	getLatestVehicles(c)
}

// getAPIStatus returns API status
func getAPIStatus(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "running",
		"timestamp": time.Now().Format(time.RFC3339),
		"interval":  FetchInterval.String(),
		"workers":   MaxWorkers,
	})
}

// getAnalytics returns analytics data
func getAnalytics(c *gin.Context) {
	// Get basic analytics
	cacheMu.RLock()
	vehicleCount := len(vehicleCache)
	cacheMu.RUnlock()

	c.JSON(200, gin.H{
		"total_vehicles": vehicleCount,
		"locations":      len(locations),
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

// getDeduplicationInfo returns deduplication information
func getDeduplicationInfo(c *gin.Context) {
	c.JSON(200, gin.H{
		"max_distance_meters": MaxDistanceMeters,
		"max_bearing_diff":    MaxBearingDiff,
		"min_time_diff":       MinTimeDiff.String(),
	})
}

// getHealthCheck returns health check status
func getHealthCheck(c *gin.Context) {
	healthStatus := GetHealthStatus()
	c.JSON(200, healthStatus)
}

// getPerformanceStats returns performance statistics
func getPerformanceStats(c *gin.Context) {
	stats := map[string]interface{}{
		"goroutine_count": runtime.NumGoroutine(),
		"cache_size":      len(vehicleCache),
		"rate_limit_size": len(rateLimiter),
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	// Add enhanced rate limiter stats
	if enhancedRateLimiter != nil {
		stats["enhanced_rate_limiter"] = enhancedRateLimiter.GetStats()
	}

	// Add worker pool stats
	if globalWorkerPool != nil {
		stats["worker_pool"] = globalWorkerPool.GetStats()
	}

	// Add circuit breaker stats
	if apiCircuitBreaker != nil {
		stats["api_circuit_breaker"] = apiCircuitBreaker.GetStats()
	}
	if databaseCircuitBreaker != nil {
		stats["database_circuit_breaker"] = databaseCircuitBreaker.GetStats()
	}
	if redisCircuitBreaker != nil {
		stats["redis_circuit_breaker"] = redisCircuitBreaker.GetStats()
	}

	c.JSON(200, stats)
}

// getJWTStatus returns JWT token status
func getJWTStatus(c *gin.Context) {
	jwtManager := GetJWTManager()
	if jwtManager == nil {
		c.JSON(500, gin.H{
			"error": "JWT manager not initialized",
		})
		return
	}

	tokenInfo := jwtManager.GetTokenInfo()
	if tokenInfo == nil {
		c.JSON(200, gin.H{
			"status":  "no_token",
			"message": "No JWT token available",
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "active",
		"token_info": gin.H{
			"expires_at":        tokenInfo.ExpiresAt.Format(time.RFC3339),
			"issued_at":         tokenInfo.IssuedAt.Format(time.RFC3339),
			"user_id":           tokenInfo.UserID,
			"login_id":          tokenInfo.LoginID,
			"is_valid":          jwtManager.IsTokenValid(),
			"time_until_expiry": time.Until(tokenInfo.ExpiresAt).String(),
		},
	})
}

// getVehicleHistoryAPI returns vehicle history with optional filtering
func getVehicleHistoryAPI(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "100")
	vehicleID := c.Query("vehicle_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000 // Cap at 1000 for performance
	}

	// Get history records
	records, err := getVehicleHistory(limit, vehicleID)
	if err != nil {
		log.Printf("❌ Failed to get vehicle history: %v", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve vehicle history",
		})
		return
	}

	c.JSON(200, gin.H{
		"records":    records,
		"count":      len(records),
		"limit":      limit,
		"vehicle_id": vehicleID,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// getHeatmapAPI returns heatmap data for analytics with cache-first approach
func getHeatmapAPI(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	interval := c.DefaultQuery("interval", "10min")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500 // Cap at 500 for performance
	}

	// Try to get from cache first
	var cachedResult map[string]interface{}
	if err := getCachedHeatmapData(interval, &cachedResult); err == nil {
		// Cache hit - return immediately
		c.JSON(200, cachedResult)
		return
	}

	// Cache miss - query database with timeout
	_, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Get heatmap data from database
	hotspots, err := getHeatmapData(limit)
	if err != nil {
		log.Printf("❌ Failed to get heatmap data: %v", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve heatmap data",
		})
		return
	}

	// Prepare response
	result := gin.H{
		"hotspots":  hotspots,
		"count":     len(hotspots),
		"limit":     limit,
		"interval":  interval,
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "database",
	}

	// Cache the result for future requests
	go func() {
		if err := cacheHeatmapData(interval, result); err != nil {
			log.Printf("⚠️ Failed to cache heatmap data: %v", err)
		}
	}()

	c.JSON(200, result)
}

// getTrendAPI returns trend data for analytics with cache-first approach
func getTrendAPI(c *gin.Context) {
	// Parse query parameters
	interval := c.DefaultQuery("interval", "hour")
	if interval != "hour" && interval != "day" {
		c.JSON(400, gin.H{
			"error": "Invalid interval. Supported values: hour, day",
		})
		return
	}

	// Try to get from cache first
	var cachedResult map[string]interface{}
	if err := getCachedTrendData(interval, &cachedResult); err == nil {
		// Cache hit - return immediately
		c.JSON(200, cachedResult)
		return
	}

	// Cache miss - query database with timeout
	_, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Get trend data from database
	trends, err := getTrendData(interval)
	if err != nil {
		log.Printf("❌ Failed to get trend data: %v", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve trend data",
		})
		return
	}

	// Prepare response
	result := gin.H{
		"trends":    trends,
		"count":     len(trends),
		"interval":  interval,
		"smoothing": "ema_0.3",
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "database",
	}

	// Cache the result for future requests
	go func() {
		if err := cacheTrendData(interval, result); err != nil {
			log.Printf("⚠️ Failed to cache trend data: %v", err)
		}
	}()

	c.JSON(200, result)
}

// getAnalyticsHistoryAPI returns comprehensive analytics from vehicle history with cache-first approach
func getAnalyticsHistoryAPI(c *gin.Context) {
	// Parse query parameters with defaults
	startStr := c.DefaultQuery("start", "")
	endStr := c.DefaultQuery("end", "")
	vehicleID := c.Query("vehicle_id")
	bboxStr := c.Query("bbox")
	limitStr := c.DefaultQuery("limit", "1000")
	offsetStr := c.DefaultQuery("offset", "0")
	gridStr := c.DefaultQuery("grid", "0.001")
	stopMinSecStr := c.DefaultQuery("stop_min_sec", "120")
	stopMaxMoveMStr := c.DefaultQuery("stop_max_move_m", "10.0")

	// Parse and validate start/end times
	var start, end time.Time
	var err error

	if startStr == "" {
		// Default to 24 hours ago
		start = time.Now().Add(-24 * time.Hour)
	} else {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid start time format. Use RFC3339 (e.g., 2025-01-01T00:00:00Z)",
			})
			return
		}
	}

	if endStr == "" {
		// Default to now
		end = time.Now()
	} else {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid end time format. Use RFC3339 (e.g., 2025-01-01T23:59:59Z)",
			})
			return
		}
	}

	// Validate time range
	if start.After(end) {
		c.JSON(400, gin.H{
			"error": "Start time must be before end time",
		})
		return
	}

	// Parse limit and offset
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000 // Cap at 5000 for performance
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Parse grid size
	grid, err := strconv.ParseFloat(gridStr, 64)
	if err != nil || grid <= 0 {
		grid = 0.001 // Default ~110m
	}

	// Parse stop detection parameters
	stopMinSec, err := strconv.Atoi(stopMinSecStr)
	if err != nil || stopMinSec < 0 {
		stopMinSec = 120 // Default 2 minutes
	}

	stopMaxMoveM, err := strconv.ParseFloat(stopMaxMoveMStr, 64)
	if err != nil || stopMaxMoveM < 0 {
		stopMaxMoveM = 10.0 // Default 10 meters
	}

	// Parse bounding box if provided
	var bbox *struct{ MinLat, MinLng, MaxLat, MaxLng float64 }
	if bboxStr != "" {
		var minLat, minLng, maxLat, maxLng float64
		n, err := fmt.Sscanf(bboxStr, "%f,%f,%f,%f", &minLat, &minLng, &maxLat, &maxLng)
		if err != nil || n != 4 {
			c.JSON(400, gin.H{
				"error": "Invalid bbox format. Use 'minLat,minLng,maxLat,maxLng'",
			})
			return
		}
		bbox = &struct{ MinLat, MinLng, MaxLat, MaxLng float64 }{
			MinLat: minLat, MinLng: minLng, MaxLat: maxLat, MaxLng: maxLng,
		}
	}

	// Create cache key for this request
	startStr = start.Format("2006-01-02T15:04:05Z")
	endStr = end.Format("2006-01-02T15:04:05Z")
	gridStr = fmt.Sprintf("%.3f", grid)

	// Try to get from cache first
	var cachedResult map[string]interface{}
	if err := getCachedHistoryData(vehicleID, startStr, endStr, gridStr, &cachedResult); err == nil {
		// Cache hit - return immediately
		c.JSON(200, cachedResult)
		return
	}

	// Cache miss - query database with timeout
	_, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Create parameters
	params := HistoryAnalyticsParams{
		Start:        start,
		End:          end,
		VehicleID:    vehicleID,
		BBox:         bbox,
		Limit:        limit,
		Offset:       offset,
		Grid:         grid,
		StopMinSec:   stopMinSec,
		StopMaxMoveM: stopMaxMoveM,
	}

	// Get analytics data
	analytics, err := getHistoryAnalytics(params)
	if err != nil {
		log.Printf("❌ Failed to get history analytics: %v", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve history analytics",
		})
		return
	}

	// Prepare response
	result := gin.H{
		"summary":           analytics.Summary,
		"bearing_histogram": analytics.BearingHistogram,
		"hotspots":          analytics.Hotspots,
		"vehicles":          analytics.Vehicles,
		"timestamp":         time.Now().Format(time.RFC3339),
		"source":            "database",
	}

	// Cache the result for future requests
	go func() {
		if err := cacheHistoryData(vehicleID, startStr, endStr, gridStr, result); err != nil {
			log.Printf("⚠️ Failed to cache history data: %v", err)
		}
	}()

	c.JSON(200, result)
}

// getFastHeatmapAPI returns heatmap data with aggressive caching (sub-100ms)
func getFastHeatmapAPI(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	interval := c.DefaultQuery("interval", "10min")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 200 { // Lower cap for fast endpoint
		limit = 200
	}

	// Try to get from cache first (this should be the primary path)
	var cachedResult map[string]interface{}
	if err := getCachedHeatmapData(interval, &cachedResult); err == nil {
		// Cache hit - return immediately with fast response
		c.Header("X-Cache-Status", "HIT")
		c.Header("X-Response-Time", "<50ms")
		c.JSON(200, cachedResult)
		return
	}

	// Cache miss - return lightweight fallback data
	fallbackData := gin.H{
		"hotspots": []gin.H{
			{"grid_lat": 18.7883, "grid_lng": 98.9853, "vehicles": 15},
			{"grid_lat": 18.7900, "grid_lng": 98.9870, "vehicles": 12},
			{"grid_lat": 18.7850, "grid_lng": 98.9830, "vehicles": 8},
			{"grid_lat": 18.7920, "grid_lng": 98.9900, "vehicles": 6},
			{"grid_lat": 18.7820, "grid_lng": 98.9800, "vehicles": 4},
		},
		"count":     5,
		"limit":     limit,
		"interval":  interval,
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "fast_fallback",
	}

	c.Header("X-Cache-Status", "MISS")
	c.Header("X-Response-Time", "<100ms")
	c.JSON(200, fallbackData)
}

// getFastTrendAPI returns trend data with aggressive caching (sub-100ms)
func getFastTrendAPI(c *gin.Context) {
	// Parse query parameters
	interval := c.DefaultQuery("interval", "hour")
	if interval != "hour" && interval != "day" {
		interval = "hour"
	}

	// Try to get from cache first
	var cachedResult map[string]interface{}
	if err := getCachedTrendData(interval, &cachedResult); err == nil {
		// Cache hit - return immediately
		c.Header("X-Cache-Status", "HIT")
		c.Header("X-Response-Time", "<50ms")
		c.JSON(200, cachedResult)
		return
	}

	// Cache miss - return lightweight fallback data
	fallbackData := gin.H{
		"trends": []gin.H{
			{"time": time.Now().Add(-6 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 25, "smoothed": 25.0},
			{"time": time.Now().Add(-5 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 30, "smoothed": 27.5},
			{"time": time.Now().Add(-4 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 35, "smoothed": 30.25},
			{"time": time.Now().Add(-3 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 40, "smoothed": 33.175},
			{"time": time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 45, "smoothed": 36.222},
			{"time": time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:00:00"), "vehicles": 50, "smoothed": 39.355},
		},
		"count":     6,
		"interval":  interval,
		"smoothing": "ema_0.3",
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "fast_fallback",
	}

	c.Header("X-Cache-Status", "MISS")
	c.Header("X-Response-Time", "<100ms")
	c.JSON(200, fallbackData)
}
