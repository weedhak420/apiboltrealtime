package main

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (cs CircuitState) String() string {
	switch cs {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures      int
	timeout          time.Duration
	failures         int
	lastFailure      time.Time
	state            CircuitState
	mutex            sync.RWMutex
	successCount     int
	halfOpenMaxCalls int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:      maxFailures,
		timeout:          timeout,
		state:            StateClosed,
		halfOpenMaxCalls: 3, // Allow 3 calls in half-open state
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Check if circuit is open
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) > cb.timeout {
			// Timeout has passed, move to half-open
			cb.state = StateHalfOpen
			cb.successCount = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	// Check if we're in half-open state and have reached max calls
	if cb.state == StateHalfOpen && cb.successCount >= cb.halfOpenMaxCalls {
		return fmt.Errorf("circuit breaker is half-open and max calls reached")
	}

	// Execute the function
	err := fn()

	if err != nil {
		// Function failed
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == StateHalfOpen {
			// Failed in half-open state, go back to open
			cb.state = StateOpen
			cb.successCount = 0
		} else if cb.failures >= cb.maxFailures {
			// Too many failures, open the circuit
			cb.state = StateOpen
		}

		return err
	}

	// Function succeeded
	cb.failures = 0

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.halfOpenMaxCalls {
			// Enough successes, close the circuit
			cb.state = StateClosed
			cb.successCount = 0
		}
	}

	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return map[string]interface{}{
		"state":           cb.state.String(),
		"failures":        cb.failures,
		"max_failures":    cb.maxFailures,
		"timeout_seconds": cb.timeout.Seconds(),
		"last_failure":    cb.lastFailure,
		"success_count":   cb.successCount,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successCount = 0
	cb.lastFailure = time.Time{}
}

// Global circuit breakers for different services
var (
	apiCircuitBreaker      *CircuitBreaker
	databaseCircuitBreaker *CircuitBreaker
	redisCircuitBreaker    *CircuitBreaker
)

// InitializeCircuitBreakers initializes global circuit breakers
func InitializeCircuitBreakers() {
	// API circuit breaker: 5 failures in 30 seconds
	apiCircuitBreaker = NewCircuitBreaker(5, 30*time.Second)

	// Database circuit breaker: 3 failures in 60 seconds
	databaseCircuitBreaker = NewCircuitBreaker(3, 60*time.Second)

	// Redis circuit breaker: 3 failures in 30 seconds
	redisCircuitBreaker = NewCircuitBreaker(3, 30*time.Second)
}

// GetAPICircuitBreaker returns the API circuit breaker
func GetAPICircuitBreaker() *CircuitBreaker {
	return apiCircuitBreaker
}

// GetDatabaseCircuitBreaker returns the database circuit breaker
func GetDatabaseCircuitBreaker() *CircuitBreaker {
	return databaseCircuitBreaker
}

// GetRedisCircuitBreaker returns the Redis circuit breaker
func GetRedisCircuitBreaker() *CircuitBreaker {
	return redisCircuitBreaker
}
