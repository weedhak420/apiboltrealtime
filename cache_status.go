package main

import (
	"time"

	"github.com/gin-gonic/gin"
)

// getCacheStatus returns cache status and statistics
func getCacheStatus(c *gin.Context) {
	status := gin.H{
		"redis_connected":  redisClient != nil,
		"analytics_worker": analyticsWorker != nil,
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	// Check Redis connection
	if redisClient != nil {
		_, err := redisClient.Ping(ctx).Result()
		status["redis_ping"] = err == nil
		if err != nil {
			status["redis_error"] = err.Error()
		}
	}

	// Check analytics worker status
	if analyticsWorker != nil {
		status["worker_running"] = true
	} else {
		status["worker_running"] = false
	}

	// Get cache keys count
	if redisClient != nil {
		keys, err := redisClient.Keys(ctx, "analytics:*").Result()
		if err == nil {
			status["cache_keys_count"] = len(keys)
			status["cache_keys"] = keys
		}
	}

	c.JSON(200, status)
}
