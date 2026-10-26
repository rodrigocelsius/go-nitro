// Package amqp implements queue.Queue on RabbitMQ (topic exchange).
package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

const (
	headerAttempts = "x-nitro-attempts"
	headerMaxRetry = "x-nitro-max-retry"
	headerMsgID    = "x-nitro-id"
)

// Config configures the RabbitMQ adapter.
type Config struct {
	URL          string
	Exchange     string
	ExchangeType string // default topic
	Prefetch     int
	RetryMax     int
	// RetryDelay is the base delay for consumer reconnect backoff.
	// It grows exponentially per consecutive failure up to MaxBackoff (default 30s).
	RetryDelay time.Duration
	// MaxBackoff caps the reconnect backoff; 0 defaults to 30s.
	MaxBackoff time.Duration
	Durable    bool
	// DLQSuffix, when non-empty, declares queue+"."+DLQSuffix and dead-letters exhausted messages.
	DLQSuffix string
}

// DefaultConfig returns sensible defaults for local/dev.
func DefaultConfig() Config {
	return Config{
		URL:          "amqp://guest:guest@127.0.0.1:5672/",
		Exchange:     "nitro.events",
		ExchangeType: "topic",
		Prefetch:     5,
		RetryMax:     3,
		RetryDelay:   time.Second,
		MaxBackoff:   30 * time.Second,
		Durable:      true,
		DLQSuffix:    "dlq",
	}
}

type subscription struct {
	queue      string
	routingKey string
	handler    queue.Handler
}

// Client is a RabbitMQ-backed queue.Queue.
type Client struct {
	cfg     Config
	conn    *amqp.Connection
	pubCh   *amqp.Channel
	mu      sync.Mutex
	subs    []subscription
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started atomic.Bool
	stats   map[string]*queue.Stats
}

// Open dials RabbitMQ and declares the exchange.
func Open(cfg Config) (*Client, error) {
	def := DefaultConfig()
	if cfg.URL == "" {
		cfg.URL = def.URL
	}
	if cfg.Exchange == "" {
		cfg.Exchange = def.Exchange
	}
	if cfg.ExchangeType == "" {
		cfg.ExchangeType = def.ExchangeType
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = def.Prefetch
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = def.RetryMax
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = def.RetryDelay
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = def.MaxBackoff
	}
	if cfg.MaxBackoff < cfg.RetryDelay {
		cfg.MaxBackoff = cfg.RetryDelay
	}
	// DLQSuffix: zero value → default "dlq"; "-" disables DLQ.
	switch cfg.DLQSuffix {
	case "":
		cfg.DLQSuffix = def.DLQSuffix
	case "-":
		cfg.DLQSuffix = ""
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("amqp: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqp: channel: %w", err)
	}
	if err := declareExchange(ch, &cfg); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("amqp: confirm mode: %w", err)
	}
	return &Client{
		cfg:   cfg,
		conn:  conn,
		pubCh: ch,
		stats: make(map[string]*queue.Stats),
	}, nil
}

func declareExchange(ch *amqp.Channel, cfg *Config) error {
	if err := ch.ExchangeDeclare(cfg.Exchange, cfg.ExchangeType, cfg.Durable, false, false, false, nil); err != nil {
		return fmt.Errorf("amqp: exchange declare: %w", err)
	}
	return nil
}

// dialLocked dials a fresh connection and publisher channel (confirm mode on)
// and declares the exchange. Callers must hold c.mu.
func (c *Client) dialLocked() error {
	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("amqp: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("amqp: channel: %w", err)
	}
	if err := declareExchange(ch, &c.cfg); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("amqp: confirm mode: %w", err)
	}
	if c.conn != nil && !c.conn.IsClosed() {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.pubCh = ch
	return nil
}

// reconnect replaces a dead connection/publisher channel. Safe to call from
// concurrent supervisors: the first one wins, the rest see a live connection.
func (c *Client) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.conn.IsClosed() && c.pubCh != nil && !c.pubCh.IsClosed() {
		return nil // already reconnected by another supervisor
	}
	return c.dialLocked()
}

// Close closes the underlying connection (also called by Stop).
func (c *Client) Close() error {
	_ = c.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pubCh != nil {
		_ = c.pubCh.Close()
		c.pubCh = nil
	}
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Publish implements queue.Queue — queue is the routing key.
func (c *Client) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	return c.PublishWithOptions(ctx, routingKey, payload)
}

// PublishWithOptions implements queue.MetaPublisher.
func (c *Client) PublishWithOptions(ctx context.Context, routingKey string, payload interface{}, opts ...queue.PublishOption) error {
	po := queue.ApplyPublishOptions(opts...)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("amqp: marshal: %w", err)
	}
	maxRetry := c.cfg.RetryMax
	if po.MaxRetry > 0 {
		maxRetry = po.MaxRetry
	}
	id := newMsgID()
	attempts := 0
	headers := amqp.Table{
		headerMsgID:    id,
		headerMaxRetry: int32(maxRetry),
	}
	for k, v := range po.Metadata {
		if k == headerAttempts {
			attempts = asInt(v)
			continue
		}
		if k == headerMsgID && v != "" {
			id = v
			continue
		}
		headers[k] = v
	}
	headers[headerAttempts] = int32(attempts)
	headers[headerMsgID] = id

	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    id,
		Timestamp:    time.Now().UTC(),
		Headers:      headers,
		Body:         body,
	}
	dc, err := c.publish(ctx, routingKey, msg)
	if err != nil && c.publisherDead() {
		// Publish-only processes have no consumer supervisor to reconnect
		// them: redial once here so a broker restart heals on the next send.
		if rerr := c.reconnect(); rerr == nil {
			dc, err = c.publish(ctx, routingKey, msg)
		}
	}
	if err != nil {
		return fmt.Errorf("amqp: publish: %w", err)
	}
	// Publisher confirm: only report success after the broker acks the message,
	// so a silent drop (connection loss before persist) surfaces as an error.
	// Waited outside c.mu so concurrent publishers pipeline their confirms.
	if dc != nil {
		ok, err := dc.WaitContext(ctx)
		if err != nil {
			return fmt.Errorf("amqp: publish confirm: %w", err)
		}
		if !ok {
			return fmt.Errorf("amqp: publish nacked by broker")
		}
	}
	c.mu.Lock()
	st := c.ensureStats(routingKey)
	st.Pending++
	st.Total++
	c.mu.Unlock()
	return nil
}

// publish sends msg on the publisher channel (confirm mode) under c.mu.
func (c *Client) publish(ctx context.Context, routingKey string, msg amqp.Publishing) (*amqp.DeferredConfirmation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pubCh == nil {
		return nil, errClosed
	}
	return c.pubCh.PublishWithDeferredConfirmWithContext(ctx, c.cfg.Exchange, routingKey, false, false, msg)
}

// publisherDead reports a lost connection/channel on a client not Closed.
func (c *Client) publisherDead() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pubCh == nil {
		return false // Close() was called: do not resurrect
	}
	return c.conn == nil || c.conn.IsClosed() || c.pubCh.IsClosed()
}

var errClosed = errors.New("amqp: closed")

// Subscribe implements queue.Queue — binds queue to exchange with routing key = queue name.
func (c *Client) Subscribe(queueName string, handler queue.Handler) error {
	return c.SubscribeRoute(queueName, queueName, handler)
}

// SubscribeRoute implements queue.RouteSubscriber.
func (c *Client) SubscribeRoute(queueName, routingKey string, handler queue.Handler) error {
	if queueName == "" {
		return fmt.Errorf("amqp: queue name required")
	}
	if handler == nil {
		return fmt.Errorf("amqp: handler required")
	}
	if routingKey == "" {
		routingKey = queueName
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = append(c.subs, subscription{queue: queueName, routingKey: routingKey, handler: handler})
	c.ensureStats(queueName)
	return nil
}

// Start begins consuming all registered subscriptions. Each subscription runs
// under a supervisor that re-declares and re-consumes automatically when the
// channel or connection drops (exponential backoff from RetryDelay).
func (c *Client) Start(ctx context.Context) error {
	if !c.started.CompareAndSwap(false, true) {
		return fmt.Errorf("amqp: already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.mu.Lock()
	subs := append([]subscription(nil), c.subs...)
	c.mu.Unlock()

	for _, sub := range subs {
		c.wg.Add(1)
		go c.superviseSubscription(runCtx, sub)
	}
	return nil
}

// defaultStopTimeout bounds Stop: a stuck in-flight handler must not hang
// process shutdown forever.
const defaultStopTimeout = 10 * time.Second

// Stop cancels consumers and waits up to defaultStopTimeout for them to finish.
func (c *Client) Stop() error {
	if !c.started.Load() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(defaultStopTimeout):
		c.started.Store(false)
		return fmt.Errorf("amqp: stop timed out after %v (in-flight handler stuck)", defaultStopTimeout)
	}
	c.started.Store(false)
	return nil
}

// superviseSubscription keeps a consumer alive: setup → consume → on failure
// wait with exponential backoff → reconnect → repeat, until ctx is cancelled.
func (c *Client) superviseSubscription(ctx context.Context, sub subscription) {
	defer c.wg.Done()
	backoff := c.cfg.RetryDelay
	for {
		if ctx.Err() != nil {
			return
		}
		runErr := c.runSubscription(ctx, sub)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[amqp] consumer for queue %q stopped (%v); reconnecting in %v", sub.queue, runErr, backoff)
		if !sleepCtx(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, c.cfg.MaxBackoff)
	}
}

// runSubscription performs the full consumer setup (declare/bind/qos/consume)
// so it self-heals after both channel loss and broker restarts. It blocks until
// consumption ends or ctx is cancelled; returns the failure reason otherwise.
func (c *Client) runSubscription(ctx context.Context, sub subscription) error {
	conn := c.connection()
	if conn == nil || conn.IsClosed() {
		if err := c.reconnect(); err != nil {
			return err
		}
		conn = c.connection()
	}
	if err := c.declareAndBind(sub.queue, sub.routingKey); err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("amqp: consume channel: %w", err)
	}
	defer ch.Close()
	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("amqp: qos: %w", err)
	}
	deliveries, err := ch.Consume(sub.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("amqp: consume: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.consumeLoop(ctx, ch, sub, deliveries)
	}()

	select {
	case <-ctx.Done():
		select {
		case <-done:
		case <-time.After(5 * time.Second): // handler stuck; do not block Stop forever
		}
		return ctx.Err()
	case <-done:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("delivery channel closed")
	}
}

// connection snapshots the current connection under lock.
func (c *Client) connection() *amqp.Connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// sleepCtx waits d or until ctx is done; reports whether the full delay elapsed.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// nextBackoff doubles the current delay up to max, adding ±20% jitter.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		next = max
	}
	jitter := time.Duration(randInt63n(int64(next)/5 + 1))
	if randIntn(2) == 0 {
		jitter = -jitter
	}
	out := next + jitter
	if out <= 0 || out > max {
		return max
	}
	return out
}

// Stats implements queue.Queue (best-effort local counters).
func (c *Client) Stats(ctx context.Context, queueName string) (*queue.Stats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.stats[queueName]
	if !ok {
		return &queue.Stats{}, nil
	}
	cp := *st
	return &cp, nil
}

func (c *Client) ensureStats(name string) *queue.Stats {
	st, ok := c.stats[name]
	if !ok {
		st = &queue.Stats{}
		c.stats[name] = st
	}
	return st
}

func (c *Client) declareAndBind(queueName, routingKey string) error {
	conn := c.connection()
	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("amqp: connection closed")
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("amqp: declare channel: %w", err)
	}
	defer ch.Close()

	args := amqp.Table{}
	if c.cfg.DLQSuffix != "" {
		dlq := queueName + "." + c.cfg.DLQSuffix
		if _, err := ch.QueueDeclare(dlq, c.cfg.Durable, false, false, false, nil); err != nil {
			return fmt.Errorf("amqp: declare dlq: %w", err)
		}
		args["x-dead-letter-exchange"] = ""
		args["x-dead-letter-routing-key"] = dlq
	}
	if _, err := ch.QueueDeclare(queueName, c.cfg.Durable, false, false, false, args); err != nil {
		return fmt.Errorf("amqp: declare queue: %w", err)
	}
	if err := ch.QueueBind(queueName, routingKey, c.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("amqp: bind: %w", err)
	}
	return nil
}

func (c *Client) consumeLoop(ctx context.Context, ch *amqp.Channel, sub subscription, deliveries <-chan amqp.Delivery) {
	defer c.wg.Done()
	defer ch.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			c.handleDelivery(ctx, sub, d)
		}
	}
}

func (c *Client) handleDelivery(ctx context.Context, sub subscription, d amqp.Delivery) {
	msg := deliveryToMessage(sub.queue, d)
	c.mu.Lock()
	st := c.ensureStats(sub.queue)
	if st.Pending > 0 {
		st.Pending--
	}
	st.Processing++
	c.mu.Unlock()

	err := safeHandler(ctx, msg, sub.handler)

	c.mu.Lock()
	st = c.ensureStats(sub.queue)
	if st.Processing > 0 {
		st.Processing--
	}
	c.mu.Unlock()

	if err == nil {
		_ = d.Ack(false)
		c.mu.Lock()
		c.ensureStats(sub.queue).Completed++
		c.mu.Unlock()
		return
	}

	msg.Attempts++
	if msg.Attempts < msg.MaxRetry {
		meta := cloneMeta(msg.Metadata)
		pubErr := c.PublishWithOptions(ctx, sub.routingKey, json.RawMessage(msg.Payload),
			queue.WithMetadata(meta),
			queue.WithMaxRetry(msg.MaxRetry),
			withAttempt(msg.Attempts),
		)
		if pubErr != nil {
			_ = d.Nack(false, true) // requeue if republish failed
			return
		}
		_ = d.Ack(false)
		c.mu.Lock()
		c.ensureStats(sub.queue).Pending++
		c.mu.Unlock()
		return
	}
	_ = d.Nack(false, false) // to DLQ if configured
	c.mu.Lock()
	c.ensureStats(sub.queue).Failed++
	c.mu.Unlock()
}

// safeHandler invokes the user handler, converting a panic into an error so a
// misbehaving consumer cannot take down the process; the delivery then follows
// the normal retry/DLQ path.
func safeHandler(ctx context.Context, msg *queue.Message, h queue.Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[amqp] handler panic on queue %q (message %s): %v", msg.Queue, msg.ID, r)
			err = fmt.Errorf("amqp: handler panic: %v", r)
		}
	}()
	return h(ctx, msg)
}

func deliveryToMessage(queueName string, d amqp.Delivery) *queue.Message {
	meta := map[string]string{}
	attempts := 0
	maxRetry := 3
	id := d.MessageId
	if d.Headers != nil {
		for k, v := range d.Headers {
			switch k {
			case headerAttempts:
				attempts = asInt(v)
			case headerMaxRetry:
				maxRetry = asInt(v)
			case headerMsgID:
				if s, ok := v.(string); ok && s != "" {
					id = s
				}
			default:
				meta[k] = fmt.Sprint(v)
			}
		}
	}
	if id == "" {
		id = newMsgID()
	}
	return &queue.Message{
		ID:        id,
		Queue:     queueName,
		Payload:   append(json.RawMessage(nil), d.Body...),
		State:     queue.StateProcessing,
		Attempts:  attempts,
		MaxRetry:  maxRetry,
		CreatedAt: d.Timestamp,
		UpdatedAt: time.Now().UTC(),
		Metadata:  meta,
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func cloneMeta(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func withAttempt(n int) queue.PublishOption {
	return func(o *queue.PublishOptions) {
		if o.Metadata == nil {
			o.Metadata = map[string]string{}
		}
		o.Metadata[headerAttempts] = strconv.Itoa(n)
	}
}

var msgCounter uint64

func newMsgID() string {
	n := atomic.AddUint64(&msgCounter, 1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), n)
}

func randInt63n(max int64) int64 {
	if max <= 0 {
		return 0
	}
	return rand.Int63n(max)
}

func randIntn(max int) int {
	if max <= 0 {
		return 0
	}
	return rand.Intn(max)
}
