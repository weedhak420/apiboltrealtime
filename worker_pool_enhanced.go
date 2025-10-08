package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Job represents a unit of work
type Job struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Data       interface{}            `json:"data"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
	Priority   int                    `json:"priority"` // Higher number = higher priority
	Retries    int                    `json:"retries"`
	MaxRetries int                    `json:"max_retries"`
}

// JobResult represents the result of a job
type JobResult struct {
	JobID       string                 `json:"job_id"`
	Success     bool                   `json:"success"`
	Result      interface{}            `json:"result"`
	Error       error                  `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Metadata    map[string]interface{} `json:"metadata"`
	CompletedAt time.Time              `json:"completed_at"`
}

// JobHandler represents a function that processes a job
type JobHandler func(ctx context.Context, job *Job) (*JobResult, error)

// Worker represents a worker in the pool
type Worker struct {
	ID            string
	JobChan       chan *Job
	ResultChan    chan *JobResult
	QuitChan      chan bool
	Active        bool
	CurrentJob    *Job
	JobsProcessed int64
	LastActivity  time.Time
}

// WorkerPoolMetrics represents metrics for the worker pool
type WorkerPoolMetrics struct {
	ActiveWorkers  int64
	IdleWorkers    int64
	QueuedJobs     int64
	ProcessedJobs  int64
	FailedJobs     int64
	RetriedJobs    int64
	AverageJobTime time.Duration
	TotalJobs      int64
}

// EnhancedWorkerPool represents an enhanced worker pool with dynamic scaling
type EnhancedWorkerPool struct {
	// Configuration
	minWorkers     int
	maxWorkers     int
	initialWorkers int
	jobQueueSize   int

	// Workers
	workers   map[string]*Worker
	workersMu sync.RWMutex

	// Job management
	jobQueue      chan *Job
	priorityQueue *PriorityQueue
	resultChan    chan *JobResult
	quitChan      chan bool

	// Job handlers
	handlers   map[string]JobHandler
	handlersMu sync.RWMutex

	// Metrics
	metrics   *WorkerPoolMetrics
	metricsMu sync.RWMutex

	// Scaling
	scaleUpThreshold   float64
	scaleDownThreshold float64
	scaleCheckInterval time.Duration
	lastScaleCheck     time.Time

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// State
	running bool
	mu      sync.RWMutex
}

// PriorityQueue implements a priority queue for jobs
type PriorityQueue struct {
	jobs []*Job
	mu   sync.RWMutex
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		jobs: make([]*Job, 0),
	}
}

// Push adds a job to the priority queue
func (pq *PriorityQueue) Push(job *Job) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Find insertion point (higher priority first)
	insertIndex := len(pq.jobs)
	for i, existingJob := range pq.jobs {
		if job.Priority > existingJob.Priority {
			insertIndex = i
			break
		}
	}

	// Insert job
	pq.jobs = append(pq.jobs[:insertIndex], append([]*Job{job}, pq.jobs[insertIndex:]...)...)
}

// Pop removes and returns the highest priority job
func (pq *PriorityQueue) Pop() *Job {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.jobs) == 0 {
		return nil
	}

	job := pq.jobs[0]
	pq.jobs = pq.jobs[1:]
	return job
}

// Len returns the number of jobs in the queue
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.jobs)
}

// Global enhanced worker pool
var (
	globalEnhancedWorkerPool *EnhancedWorkerPool
	workerPoolMu             sync.Mutex
)

// NewEnhancedWorkerPool creates a new enhanced worker pool
func NewEnhancedWorkerPool(config *APIConfig) *EnhancedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	wp := &EnhancedWorkerPool{
		minWorkers:         1,
		maxWorkers:         config.MaxWorkers * 2, // Allow scaling up to 2x max workers
		initialWorkers:     config.MaxWorkers,
		jobQueueSize:       1000,
		workers:            make(map[string]*Worker),
		jobQueue:           make(chan *Job, 1000),
		priorityQueue:      NewPriorityQueue(),
		resultChan:         make(chan *JobResult, 1000),
		quitChan:           make(chan bool),
		handlers:           make(map[string]JobHandler),
		metrics:            &WorkerPoolMetrics{},
		scaleUpThreshold:   0.8, // Scale up when 80% of workers are busy
		scaleDownThreshold: 0.3, // Scale down when less than 30% of workers are busy
		scaleCheckInterval: 30 * time.Second,
		ctx:                ctx,
		cancel:             cancel,
	}

	return wp
}

// InitGlobalWorkerPool initializes the global worker pool
func InitGlobalWorkerPool(config *APIConfig) error {
	workerPoolMu.Lock()
	defer workerPoolMu.Unlock()

	if globalEnhancedWorkerPool != nil {
		return nil
	}

	globalEnhancedWorkerPool = NewEnhancedWorkerPool(config)
	return globalEnhancedWorkerPool.Start()
}

// GetGlobalWorkerPool returns the global worker pool
func GetGlobalWorkerPool() *EnhancedWorkerPool {
	workerPoolMu.Lock()
	defer workerPoolMu.Unlock()
	return globalEnhancedWorkerPool
}

// Start starts the worker pool
func (wp *EnhancedWorkerPool) Start() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		return fmt.Errorf("worker pool is already running")
	}

	// Start initial workers
	for i := 0; i < wp.initialWorkers; i++ {
		if err := wp.addWorker(); err != nil {
			return fmt.Errorf("failed to add initial worker: %w", err)
		}
	}

	wp.running = true

	// Start job dispatcher
	go wp.jobDispatcher()

	// Start result processor
	go wp.resultProcessor()

	// Start auto-scaling
	go wp.autoScaler()

	// Start metrics collector
	go wp.metricsCollector()

	GetLogger().Info("Enhanced worker pool started",
		zap.Int("initial_workers", wp.initialWorkers),
		zap.Int("max_workers", wp.maxWorkers),
		zap.Int("job_queue_size", wp.jobQueueSize),
	)

	return nil
}

// Stop stops the worker pool
func (wp *EnhancedWorkerPool) Stop() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.running {
		return nil
	}

	wp.running = false

	// Cancel context
	wp.cancel()

	// Signal all workers to quit
	close(wp.quitChan)

	// Wait for all workers to stop
	wp.workersMu.Lock()
	for _, worker := range wp.workers {
		worker.QuitChan <- true
	}
	wp.workersMu.Unlock()

	// Close channels
	close(wp.jobQueue)
	close(wp.resultChan)

	GetLogger().Info("Enhanced worker pool stopped")
	return nil
}

// RegisterHandler registers a job handler for a specific job type
func (wp *EnhancedWorkerPool) RegisterHandler(jobType string, handler JobHandler) {
	wp.handlersMu.Lock()
	defer wp.handlersMu.Unlock()

	wp.handlers[jobType] = handler

	GetLogger().Info("Registered job handler", zap.String("job_type", jobType))
}

// SubmitJob submits a job to the worker pool
func (wp *EnhancedWorkerPool) SubmitJob(job *Job) error {
	wp.mu.RLock()
	if !wp.running {
		wp.mu.RUnlock()
		return fmt.Errorf("worker pool is not running")
	}
	wp.mu.RUnlock()

	// Set job ID if not provided
	if job.ID == "" {
		job.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	}

	// Set creation time
	job.CreatedAt = time.Now()

	// Set default max retries
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}

	// Add to priority queue
	wp.priorityQueue.Push(job)

	// Update metrics
	atomic.AddInt64(&wp.metrics.QueuedJobs, 1)
	atomic.AddInt64(&wp.metrics.TotalJobs, 1)

	GetLogger().Debug("Job submitted",
		zap.String("job_id", job.ID),
		zap.String("job_type", job.Type),
		zap.Int("priority", job.Priority),
	)

	return nil
}

// addWorker adds a new worker to the pool
func (wp *EnhancedWorkerPool) addWorker() error {
	wp.workersMu.Lock()
	defer wp.workersMu.Unlock()

	if len(wp.workers) >= wp.maxWorkers {
		return fmt.Errorf("maximum number of workers reached")
	}

	workerID := fmt.Sprintf("worker_%d", len(wp.workers))

	worker := &Worker{
		ID:           workerID,
		JobChan:      make(chan *Job, 1),
		ResultChan:   wp.resultChan,
		QuitChan:     make(chan bool, 1),
		Active:       true,
		LastActivity: time.Now(),
	}

	wp.workers[workerID] = worker

	// Start worker goroutine
	go wp.workerLoop(worker)

	// Update metrics
	atomic.AddInt64(&wp.metrics.ActiveWorkers, 1)

	GetLogger().Info("Added worker", zap.String("worker_id", workerID))

	return nil
}

// removeWorker removes a worker from the pool
func (wp *EnhancedWorkerPool) removeWorker() error {
	wp.workersMu.Lock()
	defer wp.workersMu.Unlock()

	if len(wp.workers) <= wp.minWorkers {
		return fmt.Errorf("minimum number of workers reached")
	}

	// Find an idle worker to remove
	var workerToRemove *Worker
	for _, worker := range wp.workers {
		if !worker.Active || worker.CurrentJob == nil {
			workerToRemove = worker
			break
		}
	}

	if workerToRemove == nil {
		return fmt.Errorf("no idle workers to remove")
	}

	// Signal worker to quit
	workerToRemove.QuitChan <- true
	delete(wp.workers, workerToRemove.ID)

	// Update metrics
	atomic.AddInt64(&wp.metrics.ActiveWorkers, -1)

	GetLogger().Info("Removed worker", zap.String("worker_id", workerToRemove.ID))

	return nil
}

// workerLoop is the main loop for a worker
func (wp *EnhancedWorkerPool) workerLoop(worker *Worker) {
	defer func() {
		close(worker.JobChan)
		GetLogger().Info("Worker stopped", zap.String("worker_id", worker.ID))
	}()

	for {
		select {
		case job := <-worker.JobChan:
			wp.processJob(worker, job)
		case <-worker.QuitChan:
			return
		case <-wp.ctx.Done():
			return
		}
	}
}

// processJob processes a job
func (wp *EnhancedWorkerPool) processJob(worker *Worker, job *Job) {
	start := time.Now()
	worker.CurrentJob = job
	worker.Active = true

	// Get handler for job type
	wp.handlersMu.RLock()
	handler, exists := wp.handlers[job.Type]
	wp.handlersMu.RUnlock()

	var result *JobResult
	var err error

	if !exists {
		err = fmt.Errorf("no handler registered for job type: %s", job.Type)
		result = &JobResult{
			JobID:       job.ID,
			Success:     false,
			Error:       err,
			Duration:    time.Since(start),
			CompletedAt: time.Now(),
		}
	} else {
		// Process job with timeout
		ctx, cancel := context.WithTimeout(wp.ctx, 5*time.Minute)
		result, err = handler(ctx, job)
		cancel()

		if err != nil {
			result = &JobResult{
				JobID:       job.ID,
				Success:     false,
				Error:       err,
				Duration:    time.Since(start),
				CompletedAt: time.Now(),
			}
		}
	}

	// Handle retries
	if !result.Success && job.Retries < job.MaxRetries {
		job.Retries++
		job.CreatedAt = time.Now() // Reset creation time for retry

		// Re-queue job with exponential backoff
		delay := time.Duration(job.Retries) * time.Second
		go func() {
			time.Sleep(delay)
			wp.priorityQueue.Push(job)
		}()

		atomic.AddInt64(&wp.metrics.RetriedJobs, 1)

		GetLogger().Info("Job retried",
			zap.String("job_id", job.ID),
			zap.Int("retry", job.Retries),
			zap.Duration("delay", delay),
		)
	} else {
		// Send result
		select {
		case wp.resultChan <- result:
		case <-wp.ctx.Done():
			return
		}

		// Update metrics
		if result.Success {
			atomic.AddInt64(&wp.metrics.ProcessedJobs, 1)
		} else {
			atomic.AddInt64(&wp.metrics.FailedJobs, 1)
		}

		atomic.AddInt64(&wp.metrics.QueuedJobs, -1)
	}

	// Update worker state
	worker.CurrentJob = nil
	worker.JobsProcessed++
	worker.LastActivity = time.Now()

	// Record metrics
	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.RecordWorkerPoolJob(worker.ID, "completed", time.Since(start))
	}
}

// jobDispatcher dispatches jobs from the priority queue to workers
func (wp *EnhancedWorkerPool) jobDispatcher() {
	for {
		select {
		case <-wp.ctx.Done():
			return
		default:
			// Get next job from priority queue
			job := wp.priorityQueue.Pop()
			if job == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Find an available worker
			worker := wp.findAvailableWorker()
			if worker == nil {
				// No available workers, re-queue job
				wp.priorityQueue.Push(job)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Send job to worker
			select {
			case worker.JobChan <- job:
			case <-wp.ctx.Done():
				return
			}
		}
	}
}

// findAvailableWorker finds an available worker
func (wp *EnhancedWorkerPool) findAvailableWorker() *Worker {
	wp.workersMu.RLock()
	defer wp.workersMu.RUnlock()

	for _, worker := range wp.workers {
		if worker.Active && worker.CurrentJob == nil {
			return worker
		}
	}

	return nil
}

// resultProcessor processes job results
func (wp *EnhancedWorkerPool) resultProcessor() {
	for {
		select {
		case result := <-wp.resultChan:
			wp.handleJobResult(result)
		case <-wp.ctx.Done():
			return
		}
	}
}

// handleJobResult handles a job result
func (wp *EnhancedWorkerPool) handleJobResult(result *JobResult) {
	if result.Success {
		GetLogger().Info("Job completed successfully",
			zap.String("job_id", result.JobID),
			zap.Duration("duration", result.Duration),
		)
	} else {
		GetLogger().Error("Job failed",
			zap.String("job_id", result.JobID),
			zap.Error(result.Error),
			zap.Duration("duration", result.Duration),
		)
	}
}

// autoScaler automatically scales the worker pool based on load
func (wp *EnhancedWorkerPool) autoScaler() {
	ticker := time.NewTicker(wp.scaleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.checkAndScale()
		case <-wp.ctx.Done():
			return
		}
	}
}

// checkAndScale checks load and scales workers if needed
func (wp *EnhancedWorkerPool) checkAndScale() {
	wp.workersMu.RLock()
	activeWorkers := len(wp.workers)
	busyWorkers := 0

	for _, worker := range wp.workers {
		if worker.CurrentJob != nil {
			busyWorkers++
		}
	}
	wp.workersMu.RUnlock()

	// Calculate utilization
	utilization := float64(busyWorkers) / float64(activeWorkers)

	// Scale up if utilization is high and we haven't reached max workers
	if utilization > wp.scaleUpThreshold && activeWorkers < wp.maxWorkers {
		if err := wp.addWorker(); err != nil {
			GetLogger().Error("Failed to scale up", zap.Error(err))
		} else {
			GetLogger().Info("Scaled up worker pool",
				zap.Float64("utilization", utilization),
				zap.Int("active_workers", activeWorkers+1),
			)
		}
	}

	// Scale down if utilization is low and we haven't reached min workers
	if utilization < wp.scaleDownThreshold && activeWorkers > wp.minWorkers {
		if err := wp.removeWorker(); err != nil {
			GetLogger().Error("Failed to scale down", zap.Error(err))
		} else {
			GetLogger().Info("Scaled down worker pool",
				zap.Float64("utilization", utilization),
				zap.Int("active_workers", activeWorkers-1),
			)
		}
	}
}

// metricsCollector collects and updates metrics
func (wp *EnhancedWorkerPool) metricsCollector() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.updateMetrics()
		case <-wp.ctx.Done():
			return
		}
	}
}

// updateMetrics updates worker pool metrics
func (wp *EnhancedWorkerPool) updateMetrics() {
	wp.workersMu.RLock()
	activeWorkers := len(wp.workers)
	idleWorkers := 0

	for _, worker := range wp.workers {
		if worker.CurrentJob == nil {
			idleWorkers++
		}
	}
	wp.workersMu.RUnlock()

	// Update metrics
	atomic.StoreInt64(&wp.metrics.ActiveWorkers, int64(activeWorkers))
	atomic.StoreInt64(&wp.metrics.IdleWorkers, int64(idleWorkers))
	atomic.StoreInt64(&wp.metrics.QueuedJobs, int64(wp.priorityQueue.Len()))

	// Update global metrics
	if metricsCollector := GetMetricsCollector(); metricsCollector != nil {
		metricsCollector.SetWorkerPoolMetrics(activeWorkers, int(wp.priorityQueue.Len()))
	}
}

// GetMetrics returns current worker pool metrics
func (wp *EnhancedWorkerPool) GetMetrics() *WorkerPoolMetrics {
	wp.metricsMu.RLock()
	defer wp.metricsMu.RUnlock()

	// Create a copy of metrics
	metrics := &WorkerPoolMetrics{
		ActiveWorkers: atomic.LoadInt64(&wp.metrics.ActiveWorkers),
		IdleWorkers:   atomic.LoadInt64(&wp.metrics.IdleWorkers),
		QueuedJobs:    atomic.LoadInt64(&wp.metrics.QueuedJobs),
		ProcessedJobs: atomic.LoadInt64(&wp.metrics.ProcessedJobs),
		FailedJobs:    atomic.LoadInt64(&wp.metrics.FailedJobs),
		RetriedJobs:   atomic.LoadInt64(&wp.metrics.RetriedJobs),
		TotalJobs:     atomic.LoadInt64(&wp.metrics.TotalJobs),
	}

	return metrics
}

// GetStatus returns the current status of the worker pool
func (wp *EnhancedWorkerPool) GetStatus() map[string]interface{} {
	wp.mu.RLock()
	running := wp.running
	wp.mu.RUnlock()

	wp.workersMu.RLock()
	workerCount := len(wp.workers)
	wp.workersMu.RUnlock()

	metrics := wp.GetMetrics()

	return map[string]interface{}{
		"running":      running,
		"worker_count": workerCount,
		"min_workers":  wp.minWorkers,
		"max_workers":  wp.maxWorkers,
		"metrics":      metrics,
		"queue_length": wp.priorityQueue.Len(),
	}
}
