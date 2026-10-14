package nitro

import (
	"context"
	"os"

	"github.com/rodrigocelsius/go-nitro/adapters/amqp/v2"
	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

// Queue is the messaging abstraction (in-memory or RabbitMQ).
type Queue = queue.Queue

// QueueMessage is a dequeued message.
type QueueMessage = queue.Message

// QueueHandler processes a queue message.
type QueueHandler = queue.Handler

// QueueStats holds best-effort queue counters.
type QueueStats = queue.Stats

// QueueConfig configures the in-memory queue.
type QueueConfig = queue.Config

// PublishOption configures PublishWithOptions.
type PublishOption = queue.PublishOption

// MetaPublisher publishes with metadata.
type MetaPublisher = queue.MetaPublisher

// RouteSubscriber binds a queue to a routing key (topic brokers).
type RouteSubscriber = queue.RouteSubscriber

// RabbitConfig configures the RabbitMQ adapter.
type RabbitConfig = amqp.Config

// RabbitQueue is the RabbitMQ client (implements Queue).
type RabbitQueue = amqp.Client

// WithQueueMetadata attaches metadata on publish.
func WithQueueMetadata(meta map[string]string) PublishOption {
	return queue.WithMetadata(meta)
}

// WithTenantID sets metadata tenant_id.
func WithTenantID(tenantID string) PublishOption {
	return queue.WithTenantID(tenantID)
}

// WithQueueMaxRetry sets max retry on publish.
func WithQueueMaxRetry(n int) PublishOption {
	return queue.WithMaxRetry(n)
}

// NewInternalQueue creates an in-process channel queue.
func NewInternalQueue(cfg ...QueueConfig) Queue {
	c := queue.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return queue.NewInternal(c)
}

// NewRabbitQueue dials RabbitMQ (topic exchange).
func NewRabbitQueue(cfg RabbitConfig) (*RabbitQueue, error) {
	return amqp.Open(cfg)
}

// NewRabbitQueueFromEnv reads RABBITMQ_URL / RABBITMQ_EXCHANGE / RABBITMQ_PREFETCH.
func NewRabbitQueueFromEnv() (*RabbitQueue, error) {
	cfg := amqp.DefaultConfig()
	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("RABBITMQ_EXCHANGE"); v != "" {
		cfg.Exchange = v
	}
	if v := os.Getenv("RABBITMQ_PREFETCH"); v != "" {
		var n int
		for _, c := range v {
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int(c-'0')
		}
		if n > 0 {
			cfg.Prefetch = n
		}
	}
	return amqp.Open(cfg)
}

// Publish is a convenience for q.Publish.
func Publish(ctx context.Context, q Queue, key string, payload any) error {
	return q.Publish(ctx, key, payload)
}

// PublishWithOptions publishes when q implements MetaPublisher; otherwise falls back to Publish.
func PublishWithOptions(ctx context.Context, q Queue, key string, payload any, opts ...PublishOption) error {
	if mp, ok := q.(MetaPublisher); ok {
		return mp.PublishWithOptions(ctx, key, payload, opts...)
	}
	return q.Publish(ctx, key, payload)
}

// Subscribe registers a handler.
func Subscribe(q Queue, name string, h QueueHandler) error {
	return q.Subscribe(name, h)
}

// SubscribeRoute binds queue to routingKey when supported.
func SubscribeRoute(q Queue, name, routingKey string, h QueueHandler) error {
	if rs, ok := q.(RouteSubscriber); ok {
		return rs.SubscribeRoute(name, routingKey, h)
	}
	return q.Subscribe(name, h)
}

// DefaultRabbitConfig returns adapter defaults.
func DefaultRabbitConfig() RabbitConfig {
	return amqp.DefaultConfig()
}
