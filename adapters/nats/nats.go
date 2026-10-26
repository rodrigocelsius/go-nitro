// Package nats implements queue.Queue on NATS JetStream.
//
// Zero configuration: a stream per queue (work-queue retention, file storage)
// and a durable pull consumer per subscription are created on first use, so
// `NATS_URL=nats://localhost:4222` is all an app needs. Delivery is
// at-least-once: a message is acked after the handler succeeds, redelivered
// with backoff on error, and published to "<queue>.dlq" after RetryMax
// attempts. Instances subscribing to the same queue share the work.
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

const (
	headerAttempts = "X-Nitro-Attempts"
	headerMaxRetry = "X-Nitro-Max-Retry"
	headerMsgID    = "X-Nitro-Id"
)

// Config configures the NATS adapter.
type Config struct {
	URL string // default nats://127.0.0.1:4222
	// Name identifies the connection in NATS monitoring (default OTEL_SERVICE_NAME or "nitro").
	Name string
	// RetryMax is the number of deliveries before a message goes to the DLQ (default 3).
	RetryMax int
	// RetryDelay is the base redelivery delay, doubled per attempt (default 1s).
	RetryDelay time.Duration
	// Prefetch caps unacknowledged messages per consumer (default 64).
	Prefetch int
	// AckWait is how long a handler may run before redelivery (default 30s).
	AckWait time.Duration
	// DLQSuffix names the dead-letter subject (queue+"."+suffix); "-" disables it (default "dlq").
	DLQSuffix string
	// MaxAge bounds how long unconsumed messages are kept (default 7 days).
	MaxAge time.Duration
}

// DefaultConfig returns the zero-configuration defaults.
func DefaultConfig() Config {
	name := os.Getenv("OTEL_SERVICE_NAME")
	if name == "" {
		name = "nitro"
	}
	return Config{
		URL:        nats.DefaultURL,
		Name:       name,
		RetryMax:   3,
		RetryDelay: time.Second,
		Prefetch:   64,
		AckWait:    30 * time.Second,
		DLQSuffix:  "dlq",
		MaxAge:     7 * 24 * time.Hour,
	}
}

// ConfigFromEnv reads NATS_URL / NATS_RETRY_MAX / NATS_PREFETCH over DefaultConfig.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.URL = v
	}
	if n, err := strconv.Atoi(os.Getenv("NATS_RETRY_MAX")); err == nil && n > 0 {
		cfg.RetryMax = n
	}
	if n, err := strconv.Atoi(os.Getenv("NATS_PREFETCH")); err == nil && n > 0 {
		cfg.Prefetch = n
	}
	return cfg
}

type subscription struct {
	queue, subject string
	handler        queue.Handler
}

// Client is a NATS JetStream queue.Queue.
type Client struct {
	cfg Config
	nc  *nats.Conn
	js  jetstream.JetStream

	mu       sync.Mutex
	streams  map[string]bool // subjects with an ensured stream
	subs     []subscription
	consumes []jetstream.ConsumeContext
	stats    map[string]*queue.Stats
	started  bool
}

// Open connects to NATS (fails fast when the server is unreachable, so
// nitro.QueueFromEnv can fall back to memory) and enables JetStream.
func Open(cfg Config) (*Client, error) {
	def := DefaultConfig()
	if cfg.URL == "" {
		cfg.URL = def.URL
	}
	if cfg.Name == "" {
		cfg.Name = def.Name
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = def.RetryMax
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = def.RetryDelay
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = def.Prefetch
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = def.AckWait
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = def.MaxAge
	}
	switch cfg.DLQSuffix {
	case "":
		cfg.DLQSuffix = def.DLQSuffix
	case "-":
		cfg.DLQSuffix = ""
	}
	nc, err := nats.Connect(cfg.URL,
		nats.Name(cfg.Name),
		nats.MaxReconnects(-1), // after the first connection, reconnect forever
		nats.ReconnectWait(time.Second),
		nats.Timeout(3*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats: jetstream: %w", err)
	}
	return &Client{
		cfg:     cfg,
		nc:      nc,
		js:      js,
		streams: make(map[string]bool),
		stats:   make(map[string]*queue.Stats),
	}, nil
}

// OpenEnv is Open(ConfigFromEnv()).
func OpenEnv() (*Client, error) { return Open(ConfigFromEnv()) }

// Conn exposes the underlying connection (advanced use).
func (c *Client) Conn() *nats.Conn { return c.nc }

// streamName maps a subject to a valid stream name ("orders.created" → "NITRO_ORDERS_CREATED").
func streamName(subject string) string {
	var b strings.Builder
	b.WriteString("NITRO_")
	for _, r := range strings.ToUpper(subject) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// consumerName maps a queue name to a durable consumer name.
func consumerName(q string) string { return strings.TrimPrefix(streamName(q), "NITRO_") }

func (c *Client) ensureStream(ctx context.Context, subject string) error {
	c.mu.Lock()
	ok := c.streams[subject]
	c.mu.Unlock()
	if ok {
		return nil
	}
	_, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamName(subject),
		Subjects:  []string{subject},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    c.cfg.MaxAge,
	})
	if err != nil {
		return fmt.Errorf("nats: stream %s: %w", subject, err)
	}
	c.mu.Lock()
	c.streams[subject] = true
	c.mu.Unlock()
	return nil
}

// Publish implements queue.Queue — queue is the subject.
func (c *Client) Publish(ctx context.Context, subject string, payload interface{}) error {
	return c.PublishWithOptions(ctx, subject, payload)
}

// PublishWithOptions implements queue.MetaPublisher. It returns after
// JetStream persisted the message (publish ack).
func (c *Client) PublishWithOptions(ctx context.Context, subject string, payload interface{}, opts ...queue.PublishOption) error {
	po := queue.ApplyPublishOptions(opts...)
	body, err := marshal(payload)
	if err != nil {
		return err
	}
	if err := c.ensureStream(ctx, subject); err != nil {
		return err
	}
	maxRetry := c.cfg.RetryMax
	if po.MaxRetry > 0 {
		maxRetry = po.MaxRetry
	}
	msg := nats.NewMsg(subject)
	msg.Data = body
	id := queue.NewMessageID()
	for k, v := range po.Metadata {
		if k == headerMsgID && v != "" {
			id = v
			continue
		}
		msg.Header.Set(k, v)
	}
	msg.Header.Set(headerMsgID, id)
	msg.Header.Set(headerMaxRetry, strconv.Itoa(maxRetry))
	msg.Header.Set(nats.MsgIdHdr, id) // JetStream de-duplication window
	if _, err := c.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("nats: publish: %w", err)
	}
	c.mu.Lock()
	st := c.ensureStats(subject)
	st.Pending++
	st.Total++
	c.mu.Unlock()
	return nil
}

func marshal(payload any) ([]byte, error) {
	switch v := payload.(type) {
	case json.RawMessage:
		return v, nil
	case []byte:
		return json.Marshal(v)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("nats: marshal: %w", err)
	}
	return b, nil
}

// Subscribe implements queue.Queue (subject = queue name).
func (c *Client) Subscribe(name string, handler queue.Handler) error {
	return c.SubscribeRoute(name, name, handler)
}

// SubscribeRoute implements queue.RouteSubscriber: a durable consumer named
// after queueName reads subject (routingKey). Instances with the same queue
// name share the messages.
func (c *Client) SubscribeRoute(queueName, routingKey string, handler queue.Handler) error {
	if handler == nil {
		return errors.New("nats: nil handler")
	}
	if routingKey == "" {
		routingKey = queueName
	}
	sub := subscription{queue: queueName, subject: routingKey, handler: handler}
	c.mu.Lock()
	c.subs = append(c.subs, sub)
	started := c.started
	c.mu.Unlock()
	if started {
		return c.consume(context.Background(), sub)
	}
	return nil
}

// Start implements queue.Queue: creates streams/consumers and starts consuming.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("nats: already started")
	}
	c.started = true
	subs := append([]subscription(nil), c.subs...)
	c.mu.Unlock()
	for _, s := range subs {
		if err := c.consume(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) consume(ctx context.Context, s subscription) error {
	if err := c.ensureStream(ctx, s.subject); err != nil {
		return err
	}
	cons, err := c.js.CreateOrUpdateConsumer(ctx, streamName(s.subject), jetstream.ConsumerConfig{
		Durable:       consumerName(s.queue),
		FilterSubject: s.subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       c.cfg.AckWait,
		MaxAckPending: c.cfg.Prefetch,
	})
	if err != nil {
		return fmt.Errorf("nats: consumer %s: %w", s.queue, err)
	}
	cc, err := cons.Consume(func(m jetstream.Msg) { c.handle(s, m) })
	if err != nil {
		return fmt.Errorf("nats: consume %s: %w", s.queue, err)
	}
	c.mu.Lock()
	c.consumes = append(c.consumes, cc)
	c.ensureStats(s.queue)
	c.mu.Unlock()
	return nil
}

func (c *Client) handle(s subscription, m jetstream.Msg) {
	msg := toMessage(s.queue, m)
	maxRetry := msg.MaxRetry
	if maxRetry <= 0 {
		maxRetry = c.cfg.RetryMax
	}
	c.mu.Lock()
	c.ensureStats(s.queue).Processing++
	c.mu.Unlock()

	err := safeHandler(context.Background(), msg, s.handler)

	c.mu.Lock()
	st := c.ensureStats(s.queue)
	st.Processing--
	c.mu.Unlock()
	if err == nil {
		_ = m.Ack()
		c.mu.Lock()
		st.Completed++
		c.mu.Unlock()
		return
	}
	if msg.Attempts+1 < maxRetry {
		delay := c.cfg.RetryDelay << msg.Attempts
		_ = m.NakWithDelay(delay)
		return
	}
	if c.cfg.DLQSuffix != "" {
		meta := map[string]string{"x-nitro-error": err.Error(), "x-nitro-origin": s.subject}
		if derr := c.PublishWithOptions(context.Background(), s.subject+"."+c.cfg.DLQSuffix, json.RawMessage(msg.Payload), queue.WithMetadata(meta)); derr != nil {
			log.Printf("[nats] dead-letter %q: %v (message redelivered)", s.subject, derr)
			_ = m.NakWithDelay(c.cfg.RetryDelay)
			return
		}
	}
	_ = m.Term()
	c.mu.Lock()
	st.Failed++
	c.mu.Unlock()
}

func toMessage(queueName string, m jetstream.Msg) *queue.Message {
	meta := map[string]string{}
	id := ""
	maxRetry := 0
	for k, vs := range m.Headers() {
		if len(vs) == 0 {
			continue
		}
		switch k {
		case headerMsgID:
			id = vs[0]
		case headerMaxRetry:
			maxRetry, _ = strconv.Atoi(vs[0])
		case nats.MsgIdHdr:
		default:
			meta[k] = vs[0]
		}
	}
	attempts := 0
	created := time.Now().UTC()
	if md, err := m.Metadata(); err == nil {
		attempts = int(md.NumDelivered) - 1
		created = md.Timestamp
	}
	if id == "" {
		id = queue.NewMessageID()
	}
	return &queue.Message{
		ID:        id,
		Queue:     queueName,
		Payload:   append(json.RawMessage(nil), m.Data()...),
		State:     queue.StateProcessing,
		Attempts:  attempts,
		MaxRetry:  maxRetry,
		CreatedAt: created,
		UpdatedAt: time.Now().UTC(),
		Metadata:  meta,
	}
}

func safeHandler(ctx context.Context, msg *queue.Message, h queue.Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("nats: handler panic: %v", r)
		}
	}()
	return h(ctx, msg)
}

// Stop implements queue.Queue: stops consumers (in-flight handlers finish).
func (c *Client) Stop() error {
	c.mu.Lock()
	ccs := c.consumes
	c.consumes = nil
	c.started = false
	c.mu.Unlock()
	for _, cc := range ccs {
		cc.Drain()
	}
	for _, cc := range ccs {
		<-cc.Closed()
	}
	return nil
}

// Close stops consuming and closes the connection.
func (c *Client) Close() error {
	_ = c.Stop()
	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

// Stats implements queue.Queue (best-effort local counters).
func (c *Client) Stats(_ context.Context, name string) (*queue.Stats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.stats[name]
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
