// Package queue implements Nitro's queue abstraction.
//
// The queue supports two operating modes:
//   - Internal: in-memory queue using Go channels (ideal for development)
//   - External: adapter for external queue systems (Redis, RabbitMQ, etc.)
//
// Basic usage:
//
//	q := queue.NewInternal(queue.Config{Workers: 4})
//	q.Publish(ctx, "emails", &EmailJob{To: "user@example.com"})
//	q.Subscribe("emails", func(ctx context.Context, msg *queue.Message) error {
//	    // process the job
//	    return nil
//	})
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Message represents a queue message.
type Message struct {
	ID        string            `json:"id"`
	Queue     string            `json:"queue"`
	Payload   json.RawMessage   `json:"payload"`
	State     State             `json:"state"`
	Attempts  int               `json:"attempts"`
	MaxRetry  int               `json:"max_retry"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// State represents a message's state.
type State int

const (
	StatePending State = iota
	StateProcessing
	StateCompleted
	StateFailed
	StateRetrying
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateProcessing:
		return "processing"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateRetrying:
		return "retrying"
	default:
		return "unknown"
	}
}

// Handler is the function that processes a queue message.
type Handler func(ctx context.Context, msg *Message) error

// PublishOption configures PublishWithOptions.
type PublishOption func(*PublishOptions)

// PublishOptions controls metadata and retry on publish.
type PublishOptions struct {
	Metadata map[string]string
	MaxRetry int
}

// WithMetadata attaches metadata (e.g. tenant_id) to the message.
func WithMetadata(meta map[string]string) PublishOption {
	return func(o *PublishOptions) {
		if o.Metadata == nil {
			o.Metadata = make(map[string]string, len(meta))
		}
		for k, v := range meta {
			o.Metadata[k] = v
		}
	}
}

// WithTenantID sets metadata["tenant_id"].
func WithTenantID(tenantID string) PublishOption {
	return WithMetadata(map[string]string{"tenant_id": tenantID})
}

// WithMaxRetry overrides the message MaxRetry.
func WithMaxRetry(n int) PublishOption {
	return func(o *PublishOptions) {
		o.MaxRetry = n
	}
}

// ApplyPublishOptions applies options and returns the resulting struct.
func ApplyPublishOptions(opts ...PublishOption) PublishOptions {
	var o PublishOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// Queue is the interface for a message queue.
type Queue interface {
	// Publish publishes a message to a queue (or routing key, on topic brokers).
	Publish(ctx context.Context, queue string, payload interface{}) error

	// Subscribe registers a handler to process messages from a queue.
	Subscribe(queue string, handler Handler) error

	// Start starts message processing.
	Start(ctx context.Context) error

	// Stop stops message processing.
	Stop() error

	// Stats returns queue statistics.
	Stats(ctx context.Context, queue string) (*Stats, error)
}

// MetaPublisher publishes with explicit metadata / retry.
type MetaPublisher interface {
	PublishWithOptions(ctx context.Context, queue string, payload interface{}, opts ...PublishOption) error
}

// RouteSubscriber binds a queue to a routing key (e.g. RabbitMQ topic exchange).
// If routingKey is empty, uses the queue name.
type RouteSubscriber interface {
	SubscribeRoute(queue, routingKey string, handler Handler) error
}

// Stats holds statistics for a queue.
type Stats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	Total      int64 `json:"total"`
}

// Config holds queue settings.
type Config struct {
	Workers    int           `json:"workers"`
	QueueSize  int           `json:"queue_size"`
	RetryMax   int           `json:"retry_max"`
	RetryDelay time.Duration `json:"retry_delay"`
	// PublishTimeout is how long Publish waits for room when the buffer is
	// full (backpressure) before failing with ErrQueueFull. The ctx deadline
	// wins when shorter. 0 → 1s; negative → fail immediately.
	PublishTimeout time.Duration `json:"publish_timeout"`
}

// ErrQueueFull is returned when the in-memory buffer stays full for PublishTimeout.
var ErrQueueFull = errors.New("queue: queue is full")

// DefaultConfig returns default settings.
func DefaultConfig() Config {
	return Config{
		Workers:        4,
		QueueSize:      1000,
		RetryMax:       3,
		RetryDelay:     time.Second,
		PublishTimeout: time.Second,
	}
}
