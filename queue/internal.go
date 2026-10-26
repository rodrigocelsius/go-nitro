package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Internal implements an in-memory queue using Go channels.
type Internal struct {
	config   Config
	queues   map[string]chan *Message
	handlers map[string]Handler
	stats    map[string]*Stats
	workers  map[string][]context.CancelFunc
	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
}

// defaultStopTimeout bounds Stop: a stuck handler must not hang shutdown forever.
const defaultStopTimeout = 10 * time.Second

// NewInternal creates a new internal queue.
func NewInternal(config Config) *Internal {
	if config.Workers <= 0 {
		config = DefaultConfig()
	}
	return &Internal{
		config:   config,
		queues:   make(map[string]chan *Message),
		handlers: make(map[string]Handler),
		stats:    make(map[string]*Stats),
		workers:  make(map[string][]context.CancelFunc),
	}
}

// Publish publishes a message to a queue.
func (q *Internal) Publish(ctx context.Context, queue string, payload interface{}) error {
	return q.PublishWithOptions(ctx, queue, payload)
}

// PublishWithOptions implements MetaPublisher.
func (q *Internal) PublishWithOptions(ctx context.Context, queue string, payload interface{}, opts ...PublishOption) error {
	po := ApplyPublishOptions(opts...)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: failed to marshal payload: %w", err)
	}
	maxRetry := q.config.RetryMax
	if po.MaxRetry > 0 {
		maxRetry = po.MaxRetry
	}
	now := time.Now()
	msg := &Message{
		ID:        uuid.New().String(),
		Queue:     queue,
		Payload:   payloadBytes,
		State:     StatePending,
		MaxRetry:  maxRetry,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  po.Metadata,
	}

	q.mu.Lock()
	if _, exists := q.queues[queue]; !exists {
		q.queues[queue] = make(chan *Message, q.config.QueueSize)
		q.stats[queue] = &Stats{}
		if q.started {
			q.startWorkersForQueue(queue)
		}
	}
	ch := q.queues[queue]
	st := q.stats[queue]
	st.Pending++
	st.Total++
	q.mu.Unlock()

	// Fast path: room in the buffer.
	select {
	case ch <- msg:
		return nil
	default:
	}

	// Backpressure: wait for a worker to free a slot instead of failing
	// under a burst — without holding q.mu, which workers need.
	timeout := q.config.PublishTimeout
	if timeout == 0 {
		timeout = time.Second
	}
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case ch <- msg:
			return nil
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer.C:
			err = ErrQueueFull
		}
	} else {
		err = ErrQueueFull
	}
	q.mu.Lock()
	st.Pending--
	st.Total--
	q.mu.Unlock()
	if errors.Is(err, ErrQueueFull) {
		return fmt.Errorf("%w: %s", ErrQueueFull, queue)
	}
	return err
}

// Subscribe registers a handler to process messages from a queue.
func (q *Internal) Subscribe(queue string, handler Handler) error {
	return q.SubscribeRoute(queue, queue, handler)
}

// SubscribeRoute implements RouteSubscriber (routing key ignored in-memory).
func (q *Internal) SubscribeRoute(queue, routingKey string, handler Handler) error {
	_ = routingKey
	q.mu.Lock()
	defer q.mu.Unlock()

	q.handlers[queue] = handler
	if _, exists := q.queues[queue]; exists && q.started {
		q.startWorkersForQueue(queue)
	}
	return nil
}

// Start starts message processing.
func (q *Internal) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.started {
		return fmt.Errorf("queue: already started")
	}

	q.ctx, q.cancel = context.WithCancel(ctx)
	q.started = true

	// Start workers for each existing queue
	for queue := range q.queues {
		q.startWorkersForQueue(queue)
	}

	return nil
}

// startWorkersForQueue starts workers for a specific queue.
// Must be called while holding q.mu.Lock().
func (q *Internal) startWorkersForQueue(queue string) {
	handler, exists := q.handlers[queue]
	if !exists {
		return
	}

	// Check whether workers already exist for this queue
	if _, exists := q.workers[queue]; exists {
		return
	}

	for i := 0; i < q.config.Workers; i++ {
		workerCtx, workerCancel := context.WithCancel(q.ctx)
		q.workers[queue] = append(q.workers[queue], workerCancel)
		q.wg.Add(1)
		go q.worker(workerCtx, queue, q.queues[queue], handler)
	}
}

// Stop stops message processing.
func (q *Internal) Stop() error {
	q.mu.Lock()

	if !q.started {
		q.mu.Unlock()
		return nil
	}

	q.cancel()
	for _, cancels := range q.workers {
		for _, cancel := range cancels {
			cancel()
		}
	}

	wgDone := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(wgDone)
	}()
	q.mu.Unlock()

	select {
	case <-wgDone:
	case <-time.After(defaultStopTimeout):
		return fmt.Errorf("queue: stop timed out after %v (handler stuck)", defaultStopTimeout)
	}

	q.mu.Lock()
	q.started = false
	q.workers = make(map[string][]context.CancelFunc)
	q.mu.Unlock()
	return nil
}

// Stats returns queue statistics.
func (q *Internal) Stats(ctx context.Context, queue string) (*Stats, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats, exists := q.stats[queue]
	if !exists {
		return &Stats{}, nil
	}

	// Return a copy
	return &Stats{
		Pending:    stats.Pending,
		Processing: stats.Processing,
		Completed:  stats.Completed,
		Failed:     stats.Failed,
		Total:      stats.Total,
	}, nil
}

// worker processes messages from a queue.
func (q *Internal) worker(ctx context.Context, queue string, ch <-chan *Message, handler Handler) {
	defer q.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			q.processMessage(queue, msg, handler)
		}
	}
}

// processMessage processes a single message.
func (q *Internal) processMessage(queue string, msg *Message, handler Handler) {
	q.mu.Lock()
	q.stats[queue].Pending--
	q.stats[queue].Processing++
	q.mu.Unlock()

	msg.State = StateProcessing
	msg.UpdatedAt = time.Now()

	err := invokeHandler(q.ctx, msg, handler)

	q.mu.Lock()
	defer q.mu.Unlock()

	q.stats[queue].Processing--

	if err != nil {
		msg.Attempts++
		if msg.Attempts < msg.MaxRetry {
			msg.State = StateRetrying
			msg.UpdatedAt = time.Now()
			// Requeue with delay — tracked by the WaitGroup and cancellable
			// via ctx so Stop() does not leave orphan goroutines behind.
			q.wg.Add(1)
			go func(queue string, msg *Message) {
				defer q.wg.Done()
				select {
				case <-time.After(q.config.RetryDelay):
				case <-q.ctx.Done():
					return
				}
				q.mu.Lock()
				select {
				case q.queues[queue] <- msg:
					q.stats[queue].Pending++
				default:
					q.stats[queue].Failed++
				}
				q.mu.Unlock()
			}(queue, msg)
		} else {
			msg.State = StateFailed
			msg.UpdatedAt = time.Now()
			q.stats[queue].Failed++
		}
	} else {
		msg.State = StateCompleted
		msg.UpdatedAt = time.Now()
		q.stats[queue].Completed++
	}
}

// invokeHandler invokes the user handler, converting a panic into an error so
// a misbehaving consumer cannot take down the process; the message then
// follows the normal retry path.
func invokeHandler(ctx context.Context, msg *Message, h Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[queue] handler panic on queue %q (message %s): %v", msg.Queue, msg.ID, r)
			err = fmt.Errorf("queue: handler panic: %v", r)
		}
	}()
	return h(ctx, msg)
}

// NewMessageID returns a unique message id (shared by the broker adapters).
func NewMessageID() string { return uuid.New().String() }
