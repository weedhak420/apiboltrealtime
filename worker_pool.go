package main

import (
	"log"
	"sync"
	"time"
)

// WorkerPool manages a pool of workers for concurrent processing
type WorkerPool struct {
	workers    int
	jobQueue   chan func()
	quit       chan bool
	wg         sync.WaitGroup
	activeJobs int64
	mutex      sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		jobQueue: make(chan func(), workers*2), // Buffer size is 2x workers
		quit:     make(chan bool),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	log.Printf("🚀 Starting worker pool with %d workers", wp.workers)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the main worker goroutine
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case job := <-wp.jobQueue:
			wp.mutex.Lock()
			wp.activeJobs++
			wp.mutex.Unlock()

			// Execute the job with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Worker %d panic recovered: %v", id, r)
					}

					wp.mutex.Lock()
					wp.activeJobs--
					wp.mutex.Unlock()
				}()

				job()
			}()

		case <-wp.quit:
			log.Printf("🛑 Worker %d shutting down", id)
			return
		}
	}
}

// Submit submits a job to the worker pool
func (wp *WorkerPool) Submit(job func()) bool {
	select {
	case wp.jobQueue <- job:
		return true
	default:
		// Queue is full, job rejected
		return false
	}
}

// SubmitWithTimeout submits a job with a timeout
func (wp *WorkerPool) SubmitWithTimeout(job func(), timeout time.Duration) bool {
	select {
	case wp.jobQueue <- job:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop() {
	log.Println("🛑 Stopping worker pool...")
	close(wp.quit)
	wp.wg.Wait()
	log.Println("✅ Worker pool stopped")
}

// GetStats returns statistics about the worker pool
func (wp *WorkerPool) GetStats() map[string]interface{} {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	return map[string]interface{}{
		"workers":     wp.workers,
		"queue_size":  len(wp.jobQueue),
		"queue_cap":   cap(wp.jobQueue),
		"active_jobs": wp.activeJobs,
	}
}

// Global worker pool
var globalWorkerPool *WorkerPool

// InitializeWorkerPool initializes the global worker pool
func InitializeWorkerPool(workers int) {
	globalWorkerPool = NewWorkerPool(workers)
	globalWorkerPool.Start()
}

// SubmitJob submits a job to the global worker pool
func SubmitJob(job func()) bool {
	if globalWorkerPool == nil {
		// Fallback to direct execution
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("⚠️ Job panic recovered: %v", r)
				}
			}()
			job()
		}()
		return true
	}
	return globalWorkerPool.Submit(job)
}

// SubmitJobWithTimeout submits a job with timeout to the global worker pool
func SubmitJobWithTimeout(job func(), timeout time.Duration) bool {
	if globalWorkerPool == nil {
		// Fallback to direct execution
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("⚠️ Job panic recovered: %v", r)
				}
			}()
			job()
		}()
		return true
	}
	return globalWorkerPool.SubmitWithTimeout(job, timeout)
}
