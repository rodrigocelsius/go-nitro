package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// FallbackConfig tunes Fallback.
type FallbackConfig struct {
	// BufferSize bounds messages held in memory while the primary is down
	// and no local subscriber exists for their queue (0 → 10_000).
	BufferSize int
	// RetryEvery is the interval between attempts to flush buffered
	// messages to the primary (0 → 2s).
	RetryEvery time.Duration
	// Local configures the in-process queue (zero → DefaultConfig).
	Local Config
}

type pendingMessage struct {
	queue   string
	payload json.RawMessage
	opts    PublishOptions
}

// Fallback publishes to a primary broker (RabbitMQ) and keeps working when it
// is unreachable:
//
//   - a queue with a subscriber in this process is delivered by the in-memory
//     queue (same handler, same retry rules);
//   - otherwise messages are buffered in memory (bounded) and re-published to
//     the primary, in order, as soon as it accepts them again.
//
// Subscribe registers the handler on both, so consumers keep running whichever
// side a message comes from. Buffered messages live in memory: they are lost if
// the process stops before the primary comes back (log on Stop).
type Fallback struct {
	primary Queue
	local   *Internal
	cfg     FallbackConfig

	mu      sync.Mutex
	subs    map[string]bool
	pending []pendingMessage
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	warned  bool
}

// NewFallback wraps primary (nil = in-memory only).
func NewFallback(primary Queue, cfg FallbackConfig) *Fallback {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 10_000
	}
	if cfg.RetryEvery <= 0 {
		cfg.RetryEvery = 2 * time.Second
	}
	return &Fallback{
		primary: primary,
		local:   NewInternal(cfg.Local),
		cfg:     cfg,
		subs:    make(map[string]bool),
	}
}

// Primary returns the wrapped broker (nil when running in-memory only).
func (f *Fallback) Primary() Queue { return f.primary }

// Publish implements Queue.
func (f *Fallback) Publish(ctx context.Context, queue string, payload interface{}) error {
	return f.PublishWithOptions(ctx, queue, payload)
}

// PublishWithOptions implements MetaPublisher.
func (f *Fallback) PublishWithOptions(ctx context.Context, queue string, payload interface{}, opts ...PublishOption) error {
	if f.primary == nil {
		return f.local.PublishWithOptions(ctx, queue, payload, opts...)
	}
	f.mu.Lock()
	backlog := len(f.pending) > 0
	f.mu.Unlock()
	if !backlog { // keep order: while a backlog exists new messages queue behind it
		err := publishTo(ctx, f.primary, queue, payload, opts...)
		if err == nil {
			return nil
		}
		f.warnOnce(err)
	}
	if f.hasLocal(queue) {
		return f.local.PublishWithOptions(ctx, queue, payload, opts...)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: failed to marshal payload: %w", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) >= f.cfg.BufferSize {
		return fmt.Errorf("%w: primary unavailable and fallback buffer full (%d)", ErrQueueFull, f.cfg.BufferSize)
	}
	f.pending = append(f.pending, pendingMessage{queue: queue, payload: raw, opts: ApplyPublishOptions(opts...)})
	return nil
}

func publishTo(ctx context.Context, q Queue, queue string, payload any, opts ...PublishOption) error {
	if mp, ok := q.(MetaPublisher); ok && len(opts) > 0 {
		return mp.PublishWithOptions(ctx, queue, payload, opts...)
	}
	return q.Publish(ctx, queue, payload)
}

func (f *Fallback) warnOnce(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.warned {
		f.warned = true
		log.Printf("[queue] primary broker unavailable, using in-memory fallback: %v", err)
	}
}

func (f *Fallback) hasLocal(queue string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subs[queue]
}

// Subscribe implements Queue.
func (f *Fallback) Subscribe(queue string, handler Handler) error {
	return f.SubscribeRoute(queue, queue, handler)
}

// SubscribeRoute implements RouteSubscriber.
func (f *Fallback) SubscribeRoute(queue, routingKey string, handler Handler) error {
	f.mu.Lock()
	f.subs[queue] = true
	if routingKey != "" {
		f.subs[routingKey] = true
	}
	f.mu.Unlock()
	if err := f.local.SubscribeRoute(queue, routingKey, handler); err != nil {
		return err
	}
	if routingKey != "" && routingKey != queue {
		// In memory the routing key is the queue name publishers use.
		if err := f.local.Subscribe(routingKey, handler); err != nil {
			return err
		}
	}
	if f.primary == nil {
		return nil
	}
	if rs, ok := f.primary.(RouteSubscriber); ok {
		return rs.SubscribeRoute(queue, routingKey, handler)
	}
	return f.primary.Subscribe(queue, handler)
}

// Start implements Queue: starts the local workers, the primary consumers and
// the flush loop. A primary that fails to start is logged, not fatal.
func (f *Fallback) Start(ctx context.Context) error {
	if err := f.local.Start(ctx); err != nil {
		return err
	}
	if f.primary == nil {
		return nil
	}
	if err := f.primary.Start(ctx); err != nil {
		log.Printf("[queue] primary broker start failed, consuming from the in-memory fallback only: %v", err)
	}
	loopCtx, cancel := context.WithCancel(ctx)
	f.mu.Lock()
	f.cancel = cancel
	f.mu.Unlock()
	f.wg.Add(1)
	go f.flushLoop(loopCtx)
	return nil
}

func (f *Fallback) flushLoop(ctx context.Context) {
	defer f.wg.Done()
	t := time.NewTicker(f.cfg.RetryEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f.Flush(ctx)
		}
	}
}

// Flush re-publishes buffered messages to the primary, in order, stopping at
// the first failure. It returns how many are still buffered.
func (f *Fallback) Flush(ctx context.Context) int {
	for {
		f.mu.Lock()
		if len(f.pending) == 0 || f.primary == nil {
			n := len(f.pending)
			f.mu.Unlock()
			return n
		}
		m := f.pending[0]
		f.mu.Unlock()

		opts := []PublishOption{func(o *PublishOptions) { *o = m.opts }}
		if err := publishTo(ctx, f.primary, m.queue, m.payload, opts...); err != nil {
			f.mu.Lock()
			n := len(f.pending)
			f.mu.Unlock()
			return n
		}
		f.mu.Lock()
		f.pending = f.pending[1:]
		if len(f.pending) == 0 {
			f.warned = false
			log.Printf("[queue] primary broker back, fallback buffer flushed")
		}
		f.mu.Unlock()
	}
}

// Buffered returns how many messages wait for the primary.
func (f *Fallback) Buffered() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pending)
}

// Stop implements Queue: one last flush attempt, then stops everything.
func (f *Fallback) Stop() error {
	f.mu.Lock()
	cancel := f.cancel
	f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	f.wg.Wait()
	if left := f.Flush(context.Background()); left > 0 {
		log.Printf("[queue] stopping with %d message(s) still buffered for the primary broker (lost)", left)
	}
	var errs []error
	if f.primary != nil {
		if err := f.primary.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := f.local.Stop(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Stats implements Queue: primary + local counters (Pending includes buffered).
func (f *Fallback) Stats(ctx context.Context, queue string) (*Stats, error) {
	out, _ := f.local.Stats(ctx, queue)
	if f.primary != nil {
		if p, err := f.primary.Stats(ctx, queue); err == nil && p != nil {
			out.Pending += p.Pending
			out.Processing += p.Processing
			out.Completed += p.Completed
			out.Failed += p.Failed
			out.Total += p.Total
		}
	}
	f.mu.Lock()
	for _, m := range f.pending {
		if m.queue == queue {
			out.Pending++
			out.Total++
		}
	}
	f.mu.Unlock()
	return out, nil
}
