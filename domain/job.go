package domain

import "context"

// Job is the interface for asynchronous jobs.
type Job interface {
	Execute(ctx context.Context) error
}

// JobPriority is a job's priority.
type JobPriority int

const (
	PriorityLow    JobPriority = 0
	PriorityNormal JobPriority = 5
	PriorityHigh   JobPriority = 10
)

// JobStatus represents a job's status.
type JobStatus string

const (
	JobPending    JobStatus = "pending"
	JobRunning    JobStatus = "running"
	JobCompleted  JobStatus = "completed"
	JobFailed     JobStatus = "failed"
	JobDeadLetter JobStatus = "dead_letter"
)

// JobResult represents a job's result.
type JobResult struct {
	ID       string
	Status   JobStatus
	Error    error
	Attempts int
	Priority JobPriority
}

// JobConfig configures a job's behavior.
type JobConfig struct {
	Priority   JobPriority
	MaxRetries int
	RetryDelay int // in milliseconds, grows exponentially
}

// DefaultJobConfig returns the default configuration.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		Priority:   PriorityNormal,
		MaxRetries: 3,
		RetryDelay: 100,
	}
}

// ErrDeadLetter is returned when a job has failed all attempts.
type ErrDeadLetter struct {
	JobID string
}

func (e *ErrDeadLetter) Error() string { return "nitro: job moved to dead letter queue" }
