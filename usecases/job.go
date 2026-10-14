package usecases

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Queue is the job queue interface.
type Queue interface {
	Enqueue(job domain.Job) (string, error)
	EnqueueWithConfig(job domain.Job, config domain.JobConfig) (string, error)
	Dequeue() (domain.Job, domain.JobConfig, string, error)
	Size() int
}

// Dispatcher sends jobs to the queue.
type Dispatcher struct {
	queue      Queue
	results    map[string]*domain.JobResult
	deadLetter []deadLetterEntry
	mu         sync.RWMutex
	configs    map[string]domain.JobConfig
}

type deadLetterEntry struct {
	id     string
	job    domain.Job
	config domain.JobConfig
	error  error
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(queue Queue) *Dispatcher {
	return &Dispatcher{
		queue:   queue,
		results: make(map[string]*domain.JobResult),
		configs: make(map[string]domain.JobConfig),
	}
}

// Dispatch sends a job to the queue with default configuration.
func (d *Dispatcher) Dispatch(job domain.Job) (string, error) {
	return d.DispatchWithConfig(job, domain.DefaultJobConfig())
}

// DispatchWithConfig sends a job to the queue with a specific configuration.
func (d *Dispatcher) DispatchWithConfig(job domain.Job, config domain.JobConfig) (string, error) {
	id, err := d.queue.EnqueueWithConfig(job, config)
	if err != nil {
		return "", err
	}

	d.mu.Lock()
	d.results[id] = &domain.JobResult{
		ID:       id,
		Status:   domain.JobPending,
		Priority: config.Priority,
	}
	d.configs[id] = config
	d.mu.Unlock()

	return id, nil
}

// Result returns a job result.
func (d *Dispatcher) Result(id string) (*domain.JobResult, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.results[id]
	return r, ok
}

// DeadLetter returns jobs that failed all attempts.
func (d *Dispatcher) DeadLetter() []deadLetterEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]deadLetterEntry, len(d.deadLetter))
	copy(result, d.deadLetter)
	return result
}

// processJob processes a job with retry and backoff.
func (d *Dispatcher) processJob(ctx context.Context, id string, job domain.Job, config domain.JobConfig) {
	var lastErr error
	maxAttempts := config.MaxRetries + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		d.mu.Lock()
		d.results[id] = &domain.JobResult{
			ID:       id,
			Status:   domain.JobRunning,
			Attempts: attempt,
			Priority: config.Priority,
		}
		d.mu.Unlock()

		if err := job.Execute(ctx); err != nil {
			lastErr = err
			if attempt < maxAttempts {
				// Exponential backoff: RetryDelay (ms) * 2^(attempt-1)
				base := time.Duration(config.RetryDelay) * time.Millisecond
				delay := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
				time.Sleep(delay)
				continue
			}
			// All attempts failed -> dead letter
			d.mu.Lock()
			d.results[id] = &domain.JobResult{
				ID:       id,
				Status:   domain.JobDeadLetter,
				Error:    lastErr,
				Attempts: attempt,
				Priority: config.Priority,
			}
			d.deadLetter = append(d.deadLetter, deadLetterEntry{
				id:     id,
				job:    job,
				config: config,
				error:  lastErr,
			})
			d.mu.Unlock()
			return
		}

		d.mu.Lock()
		d.results[id] = &domain.JobResult{
			ID:       id,
			Status:   domain.JobCompleted,
			Attempts: attempt,
			Priority: config.Priority,
		}
		d.mu.Unlock()
		return
	}
}

// Worker processes jobs from the queue.
type Worker struct {
	dispatcher *Dispatcher
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorker creates a new Worker.
func NewWorker(dispatcher *Dispatcher) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		dispatcher: dispatcher,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker processing jobs.
func (w *Worker) Start() {
	go func() {
		for {
			select {
			case <-w.ctx.Done():
				return
			default:
				job, config, id, err := w.dispatcher.queue.Dequeue()
				if err != nil {
					select {
					case <-w.ctx.Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
					continue
				}

				w.dispatcher.processJob(w.ctx, id, job, config)
			}
		}
	}()
}

// Stop stops the worker.
func (w *Worker) Stop() {
	w.cancel()
}
