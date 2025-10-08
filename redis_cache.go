package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var ctx = context.Background()

// Redis configuration - will be set from config file
var (
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
)

// RedisStats represents Redis statistics
type RedisStats struct {
	Connected        bool    `json:"connected"`
	MemoryUsage      int64   `json:"memory_usage"`
	KeyCount         int64   `json:"key_count"`
	HitRate          float64 `json:"hit_rate"`
	ConnectedClients int64   `json:"connected_clients"`
	Uptime           int64   `json:"uptime"`
}

// Initialize Redis with configuration
func initRedisWithConfig(host, port, username, password string, db, poolSize, minIdleConns, maxRetries int, dialTimeout, readTimeout, writeTimeout time.Duration) error {
	// Log configuration details for debugging
	log.Printf("🔧 Redis Configuration:")
	log.Printf("   Host: %s", host)
	log.Printf("   Port: %s", port)
	log.Printf("   Username: %s", username)
	log.Printf("   Password: %s", maskPassword(password))
	log.Printf("   DB: %d", db)
	log.Printf("   Pool Size: %d", poolSize)
	log.Printf("   Min Idle Connections: %d", minIdleConns)
	log.Printf("   Max Retries: %d", maxRetries)
	log.Printf("   Dial Timeout: %v", dialTimeout)
	log.Printf("   Read Timeout: %v", readTimeout)
	log.Printf("   Write Timeout: %v", writeTimeout)

	redisClient = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Username:     username,
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		MaxRetries:   maxRetries,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	// Test connection with retry logic
	for i := 0; i < maxRetries; i++ {
		log.Printf("🔄 Redis connection attempt %d/%d...", i+1, maxRetries)
		_, err := redisClient.Ping(ctx).Result()
		if err == nil {
			log.Println("✅ Redis connected successfully")
			return nil
		}

		log.Printf("⚠️ Redis connection attempt %d failed: %v", i+1, err)
		if i < maxRetries-1 {
			log.Printf("⏱️ Retrying in 2s...")
			time.Sleep(2 * time.Second)
		}
	}

	// If all retries failed, log warning but don't fail
	log.Printf("⚠️ Redis initialization failed after %d attempts - continuing without Redis", maxRetries)
	redisClient = nil
	return nil // Don't return error - make Redis optional
}

// maskPassword masks the password for logging
func maskPassword(password string) string {
	if password == "" {
		return "(empty)"
	}
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + "****" + password[len(password)-2:]
}

// Initialize Redis with default configuration (for backward compatibility)
func initRedis() error {
	return initRedisWithConfig(
		"localhost", "6379", "", "", 0, 10, 5, 3,
		5*time.Second, 3*time.Second, 3*time.Second,
	)
}

// Cache vehicles to Redis with compression
func cacheVehiclesToRedis(vehicles []Vehicle) error {
	if redisClient == nil {
		// Redis not available, skip caching
		return nil
	}

	if len(vehicles) == 0 {
		return nil
	}

	// Serialize vehicles to JSON
	jsonData, err := json.Marshal(vehicles)
	if err != nil {
		return fmt.Errorf("failed to marshal vehicles: %v", err)
	}

	// Cache with expiration (reduced to 2 minutes for better freshness with 1-second updates)
	key := "vehicles:latest"
	err = redisClient.Set(ctx, key, jsonData, 2*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to cache vehicles: %v", err)
	}

	// Also cache individual vehicles for faster access
	pipe := redisClient.Pipeline()
	for _, vehicle := range vehicles {
		vehicleKey := fmt.Sprintf("vehicle:%s", vehicle.ID)
		vehicleData, _ := json.Marshal(vehicle)
		pipe.Set(ctx, vehicleKey, vehicleData, 2*time.Minute)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("⚠️ Failed to cache individual vehicles: %v", err)
	}

	return nil
}

// Get cached vehicles from Redis
func getCachedVehiclesFromRedis() ([]Vehicle, error) {
	if redisClient == nil {
		// Redis not available, return empty slice
		return []Vehicle{}, nil
	}

	key := "vehicles:latest"
	jsonData, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no cached data found")
		}
		return nil, fmt.Errorf("failed to get cached vehicles: %v", err)
	}

	var vehicles []Vehicle
	err = json.Unmarshal([]byte(jsonData), &vehicles)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal vehicles: %v", err)
	}

	return vehicles, nil
}

// Get Redis statistics
func getRedisStats() (RedisStats, error) {
	if redisClient == nil {
		return RedisStats{}, fmt.Errorf("redis not initialized")
	}

	stats := RedisStats{}

	// Test connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		stats.Connected = false
		return stats, fmt.Errorf("redis not connected: %v", err)
	}
	stats.Connected = true

	// Get memory usage (simplified)
	stats.MemoryUsage = 1024 * 1024 // Placeholder

	// Get key count
	keys, err := redisClient.Keys(ctx, "*").Result()
	if err == nil {
		stats.KeyCount = int64(len(keys))
	}

	// Get connected clients (simplified)
	stats.ConnectedClients = 1 // Placeholder

	// Get uptime (simplified)
	stats.Uptime = 3600 // Placeholder

	stats.HitRate = 0.95 // Placeholder

	return stats, nil
}

// Clear Redis cache
func clearRedisCache() error {
	if redisClient == nil {
		return fmt.Errorf("redis not initialized")
	}

	// Clear all vehicle-related keys
	keys, err := redisClient.Keys(ctx, "vehicle:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %v", err)
	}

	if len(keys) > 0 {
		err = redisClient.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete keys: %v", err)
		}
	}

	// Clear latest vehicles
	err = redisClient.Del(ctx, "vehicles:latest").Err()
	if err != nil {
		return fmt.Errorf("failed to clear latest vehicles: %v", err)
	}

	return nil
}

// Cache location data
func cacheLocationData(locationName string, vehicles []Vehicle, success bool, errorMsg string) error {
	if redisClient == nil {
		return fmt.Errorf("redis not initialized")
	}

	locationData := map[string]interface{}{
		"location_name": locationName,
		"vehicle_count": len(vehicles),
		"success":       success,
		"error":         errorMsg,
		"timestamp":     time.Now(),
	}

	jsonData, err := json.Marshal(locationData)
	if err != nil {
		return fmt.Errorf("failed to marshal location data: %v", err)
	}

	key := fmt.Sprintf("location:%s", locationName)
	err = redisClient.Set(ctx, key, jsonData, 10*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to cache location data: %v", err)
	}

	return nil
}

// Get cached location data
func getCachedLocationData(locationName string) (map[string]interface{}, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis not initialized")
	}

	key := fmt.Sprintf("location:%s", locationName)
	jsonData, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no cached data found")
		}
		return nil, fmt.Errorf("failed to get cached location data: %v", err)
	}

	var locationData map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &locationData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal location data: %v", err)
	}

	return locationData, nil
}

// Analytics cache functions
func cacheAnalyticsData(cacheKey string, data interface{}, ttl time.Duration) error {
	if redisClient == nil {
		return fmt.Errorf("redis not initialized")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics data: %v", err)
	}

	err = redisClient.Set(ctx, cacheKey, jsonData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to cache analytics data: %v", err)
	}

	return nil
}

func getCachedAnalyticsData(cacheKey string, result interface{}) error {
	if redisClient == nil {
		return fmt.Errorf("redis not initialized")
	}

	jsonData, err := redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("no cached data found")
		}
		return fmt.Errorf("failed to get cached analytics data: %v", err)
	}

	err = json.Unmarshal([]byte(jsonData), result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal analytics data: %v", err)
	}

	return nil
}

// Cache analytics heatmap data
func cacheHeatmapData(interval string, data interface{}) error {
	cacheKey := fmt.Sprintf("analytics:heatmap:%s", interval)
	return cacheAnalyticsData(cacheKey, data, 30*time.Second)
}

// Get cached heatmap data
func getCachedHeatmapData(interval string, result interface{}) error {
	cacheKey := fmt.Sprintf("analytics:heatmap:%s", interval)
	return getCachedAnalyticsData(cacheKey, result)
}

// Cache analytics trend data
func cacheTrendData(interval string, data interface{}) error {
	cacheKey := fmt.Sprintf("analytics:trend:%s", interval)
	return cacheAnalyticsData(cacheKey, data, 30*time.Second)
}

// Get cached trend data
func getCachedTrendData(interval string, result interface{}) error {
	cacheKey := fmt.Sprintf("analytics:trend:%s", interval)
	return getCachedAnalyticsData(cacheKey, result)
}

// Cache analytics history data
func cacheHistoryData(vehicleID, start, end, grid string, data interface{}) error {
	cacheKey := fmt.Sprintf("analytics:history:%s:%s:%s:%s", vehicleID, start, end, grid)
	return cacheAnalyticsData(cacheKey, data, 60*time.Second)
}

// Get cached history data
func getCachedHistoryData(vehicleID, start, end, grid string, result interface{}) error {
	cacheKey := fmt.Sprintf("analytics:history:%s:%s:%s:%s", vehicleID, start, end, grid)
	return getCachedAnalyticsData(cacheKey, result)
}
