package main

import (
	"sync"
	"time"
)

// EnhancedRateLimiter provides advanced rate limiting with sliding window
type EnhancedRateLimiter struct {
	requests        map[string][]time.Time
	mutex           sync.RWMutex
	maxRequests     int
	window          time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan bool
}

// NewEnhancedRateLimiter creates a new rate limiter with increased limits for more locations
func NewEnhancedRateLimiter(maxRequests int, window time.Duration) *EnhancedRateLimiter {
	rl := &EnhancedRateLimiter{
		requests:        make(map[string][]time.Time),
		maxRequests:     maxRequests * 3, // Increase limit for more locations
		window:          window,
		cleanupInterval: window * 2, // Cleanup every 2 windows
		stopCleanup:     make(chan bool),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given key
func (rl *EnhancedRateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	if rl.requests[key] == nil {
		rl.requests[key] = make([]time.Time, 0)
	}

	// Remove old requests outside the window
	cutoff := now.Add(-rl.window)
	var validRequests []time.Time
	for _, req := range rl.requests[key] {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	rl.requests[key] = validRequests

	// Check if we can make another request
	if len(rl.requests[key]) >= rl.maxRequests {
		return false
	}

	// Add current request
	rl.requests[key] = append(rl.requests[key], now)
	return true
}

// GetRemainingRequests returns the number of requests remaining in the current window
func (rl *EnhancedRateLimiter) GetRemainingRequests(key string) int {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var validRequests []time.Time
	for _, req := range rl.requests[key] {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}

	return rl.maxRequests - len(validRequests)
}

// GetResetTime returns when the rate limit will reset for a key
func (rl *EnhancedRateLimiter) GetResetTime(key string) time.Time {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	if len(rl.requests[key]) == 0 {
		return time.Now()
	}

	// Find the oldest request in the current window
	oldest := rl.requests[key][0]
	for _, req := range rl.requests[key] {
		if req.Before(oldest) {
			oldest = req
		}
	}

	return oldest.Add(rl.window)
}

// cleanup removes old entries to prevent memory leaks
func (rl *EnhancedRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mutex.Lock()
			now := time.Now()
			cutoff := now.Add(-rl.window * 2) // Remove entries older than 2 windows

			for key, requests := range rl.requests {
				var validRequests []time.Time
				for _, req := range requests {
					if req.After(cutoff) {
						validRequests = append(validRequests, req)
					}
				}

				if len(validRequests) == 0 {
					delete(rl.requests, key)
				} else {
					rl.requests[key] = validRequests
				}
			}
			rl.mutex.Unlock()

		case <-rl.stopCleanup:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *EnhancedRateLimiter) Stop() {
	close(rl.stopCleanup)
}

// GetStats returns statistics about the rate limiter
func (rl *EnhancedRateLimiter) GetStats() map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	return map[string]interface{}{
		"total_keys":     len(rl.requests),
		"max_requests":   rl.maxRequests,
		"window_seconds": rl.window.Seconds(),
	}
}
