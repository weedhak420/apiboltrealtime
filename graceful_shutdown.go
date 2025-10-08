package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownManager manages graceful shutdown of the application
type ShutdownManager struct {
	shutdownChan  chan os.Signal
	shutdownFuncs []func() error
	mutex         sync.RWMutex
	shutdown      bool
	timeout       time.Duration
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	return &ShutdownManager{
		shutdownChan:  make(chan os.Signal, 1),
		shutdownFuncs: make([]func() error, 0),
		timeout:       timeout,
	}
}

// RegisterShutdownFunc registers a function to be called during shutdown
func (sm *ShutdownManager) RegisterShutdownFunc(fn func() error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.shutdownFuncs = append(sm.shutdownFuncs, fn)
}

// Start starts the shutdown manager
func (sm *ShutdownManager) Start() {
	// Register signal handlers
	signal.Notify(sm.shutdownChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start shutdown handler
	go sm.handleShutdown()
}

// handleShutdown handles shutdown signals
func (sm *ShutdownManager) handleShutdown() {
	<-sm.shutdownChan
	log.Println("🛑 Shutdown signal received, initiating graceful shutdown...")

	sm.mutex.Lock()
	sm.shutdown = true
	sm.mutex.Unlock()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	// Execute shutdown functions
	sm.executeShutdownFuncs(ctx)

	log.Println("✅ Graceful shutdown completed")
	os.Exit(0)
}

// executeShutdownFuncs executes all registered shutdown functions
func (sm *ShutdownManager) executeShutdownFuncs(ctx context.Context) {
	sm.mutex.RLock()
	funcs := make([]func() error, len(sm.shutdownFuncs))
	copy(funcs, sm.shutdownFuncs)
	sm.mutex.RUnlock()

	// Execute shutdown functions concurrently
	var wg sync.WaitGroup
	errorChan := make(chan error, len(funcs))

	for i, fn := range funcs {
		wg.Add(1)
		go func(index int, shutdownFunc func() error) {
			defer wg.Done()

			// Create a context for this specific function
			funcCtx, funcCancel := context.WithTimeout(ctx, 5*time.Second)
			defer funcCancel()

			// Execute function in a goroutine with context
			done := make(chan error, 1)
			go func() {
				done <- shutdownFunc()
			}()

			select {
			case err := <-done:
				if err != nil {
					log.Printf("⚠️ Shutdown function %d failed: %v", index, err)
					errorChan <- err
				} else {
					log.Printf("✅ Shutdown function %d completed successfully", index)
				}
			case <-funcCtx.Done():
				log.Printf("⚠️ Shutdown function %d timed out", index)
				errorChan <- funcCtx.Err()
			}
		}(i, fn)
	}

	// Wait for all functions to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ All shutdown functions completed")
	case <-ctx.Done():
		log.Println("⚠️ Shutdown timeout reached, forcing exit")
	}

	// Log any errors
	close(errorChan)
	errorCount := 0
	for err := range errorChan {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		log.Printf("⚠️ %d shutdown functions failed", errorCount)
	}
}

// IsShuttingDown returns true if the application is shutting down
func (sm *ShutdownManager) IsShuttingDown() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.shutdown
}

// Global shutdown manager
var globalShutdownManager *ShutdownManager

// InitializeShutdownManager initializes the global shutdown manager
func InitializeShutdownManager() {
	globalShutdownManager = NewShutdownManager(30 * time.Second) // 30 second timeout

	// Register shutdown functions
	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Stopping worker pool...")
		if globalWorkerPool != nil {
			globalWorkerPool.Stop()
		}
		return nil
	})

	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Stopping health monitoring...")
		if globalHealthMonitor != nil {
			globalHealthMonitor.Stop()
		}
		return nil
	})

	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Closing database connections...")
		if db != nil {
			return db.Close()
		}
		return nil
	})

	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Closing Redis connections...")
		if redisClient != nil {
			return redisClient.Close()
		}
		return nil
	})

	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Closing statement pool...")
		if globalStmtPool != nil {
			return globalStmtPool.Close()
		}
		return nil
	})

	globalShutdownManager.RegisterShutdownFunc(func() error {
		log.Println("🛑 Stopping rate limiter...")
		if enhancedRateLimiter != nil {
			enhancedRateLimiter.Stop()
		}
		return nil
	})

	// Start the shutdown manager
	globalShutdownManager.Start()
}

// GetShutdownManager returns the global shutdown manager
func GetShutdownManager() *ShutdownManager {
	return globalShutdownManager
}

// IsShuttingDown returns true if the application is shutting down
func IsShuttingDown() bool {
	if globalShutdownManager == nil {
		return false
	}
	return globalShutdownManager.IsShuttingDown()
}
