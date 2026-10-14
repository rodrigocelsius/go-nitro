// Package scheduler implements Nitro's task scheduler.
//
// The scheduler supports two operating modes:
//   - Cron: cron expressions for precise scheduling
//   - Interval: fixed intervals between runs
//
// Basic usage:
//
//	s := scheduler.New()
//	job, err := s.Every("*/5 * * * *", func(ctx context.Context) error {
//	    return nil
//	})
//	s.Start(ctx)
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Job represents a scheduled task.
type Job struct {
	Name     string
	Schedule Schedule
	Fn       Func
	LastRun  time.Time
	NextRun  time.Time
	Active   bool
}

// Func is the function executed by the job.
type Func func(ctx context.Context) error

// Schedule defines when a job should run.
type Schedule interface {
	// Next returns the next run time after a given moment.
	Next(after time.Time) time.Time

	// String returns a readable representation of the schedule.
	String() string
}

// Scheduler manages execution of scheduled tasks.
type Scheduler struct {
	jobs   []*Job
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new scheduler.
func New() *Scheduler {
	return &Scheduler{
		jobs: make([]*Job, 0),
	}
}

// Every schedules a job with an interval or cron expression.
// Supported formats:
//   - "5m" - every 5 minutes
//   - "1h" - every hour
//   - "24h" - every 24 hours
//   - "*/5 * * * *" - cron expression
func (s *Scheduler) Every(schedule string, fn Func) (*Job, error) {
	return s.ScheduleWithName("", schedule, fn)
}

// MustEvery schedules a job and panics if the expression is invalid.
// Suitable for hardcoded schedules in code.
func (s *Scheduler) MustEvery(schedule string, fn Func) *Job {
	job, err := s.Every(schedule, fn)
	if err != nil {
		panic(err)
	}
	return job
}

// ScheduleWithName schedules a job with a specific name.
// Returns an error if the schedule expression is invalid.
func (s *Scheduler) ScheduleWithName(name, schedule string, fn Func) (*Job, error) {
	sched, err := ParseSchedule(schedule)
	if err != nil {
		return nil, fmt.Errorf("scheduler: invalid schedule %q: %w", schedule, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		name = fmt.Sprintf("job_%d", len(s.jobs))
	}

	job := &Job{
		Name:     name,
		Schedule: sched,
		Fn:       fn,
		Active:   true,
	}
	s.jobs = append(s.jobs, job)
	return job, nil
}

// Start starts the scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctx, s.cancel = context.WithCancel(ctx)

	for _, job := range s.jobs {
		job.NextRun = job.Schedule.Next(time.Now())
		s.wg.Add(1)
		go s.runJob(job)
	}

	return nil
}

// defaultStopTimeout bounds Stop: a stuck job must not hang shutdown forever.
const defaultStopTimeout = 10 * time.Second

// Stop stops the scheduler, waiting up to defaultStopTimeout for running jobs.
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(defaultStopTimeout):
		return fmt.Errorf("scheduler: stop timed out after %v (job stuck)", defaultStopTimeout)
	}
	return nil
}

// Jobs returns the list of scheduled jobs.
func (s *Scheduler) Jobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Job, len(s.jobs))
	copy(result, s.jobs)
	return result
}

// runJob runs a job periodically.
func (s *Scheduler) runJob(job *Job) {
	defer s.wg.Done()

	timer := time.NewTimer(time.Until(job.NextRun))
	defer timer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
			job.LastRun = time.Now()
			s.runOnce(job)

			job.NextRun = job.Schedule.Next(time.Now())
			timer.Reset(time.Until(job.NextRun))
		}
	}
}

// runOnce executes the job function recovering from panics so a misbehaving
// job cannot kill its scheduler loop (the next run stays scheduled).
func (s *Scheduler) runOnce(job *Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler] panic in job %q: %v", job.Name, r)
		}
	}()
	_ = job.Fn(s.ctx)
}
