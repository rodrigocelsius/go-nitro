// Package kafka implements queue.Queue on Apache Kafka (and Redpanda).
//
// Zero configuration: `KAFKA_BROKERS=localhost:9092` is enough. Topics are
// created on first use (queue name = topic), each Subscribe gets its own
// consumer group (instances with the same queue name share partitions,
// different queue names on one topic each receive every message), offsets are
// committed only after the handler succeeds, failures are retried in-process
// with backoff and then published to "<topic>.dlq".
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

const (
	headerAttempts = "x-nitro-attempts"
	headerMaxRetry = "x-nitro-max-retry"
	headerMsgID    = "x-nitro-id"
)

// Config configures the Kafka adapter.
type Config struct {
	Brokers []string // default localhost:9092
	// GroupID prefixes consumer groups: "<GroupID>.<queue>" (default
	// OTEL_SERVICE_NAME or "nitro").
	GroupID string
	// RetryMax is the number of handler attempts before the DLQ (default 3).
	RetryMax int
	// RetryDelay is the base retry delay, doubled per attempt (default 1s).
	RetryDelay time.Duration
	// Partitions / ReplicationFactor for auto-created topics (default 3 / 1).
	Partitions        int
	ReplicationFactor int
	// DLQSuffix names the dead-letter topic (topic+"."+suffix); "-" disables it (default "dlq").
	DLQSuffix string
	// BatchTimeout bounds publish latency (default 10ms; kafka-go's own default is 1s).
	BatchTimeout time.Duration
}

// DefaultConfig returns the zero-configuration defaults.
func DefaultConfig() Config {
	group := os.Getenv("OTEL_SERVICE_NAME")
	if group == "" {
		group = "nitro"
	}
	return Config{
		Brokers:           []string{"localhost:9092"},
		GroupID:           group,
		RetryMax:          3,
		RetryDelay:        time.Second,
		Partitions:        3,
		ReplicationFactor: 1,
		DLQSuffix:         "dlq",
		BatchTimeout:      10 * time.Millisecond,
	}
}

// ConfigFromEnv reads KAFKA_BROKERS (comma-separated), KAFKA_GROUP_ID,
// KAFKA_RETRY_MAX and KAFKA_PARTITIONS over DefaultConfig.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		cfg.Brokers = nil
		for _, b := range strings.Split(v, ",") {
			if b = strings.TrimSpace(b); b != "" {
				cfg.Brokers = append(cfg.Brokers, b)
			}
		}
	}
	if v := os.Getenv("KAFKA_GROUP_ID"); v != "" {
		cfg.GroupID = v
	}
	if n, err := strconv.Atoi(os.Getenv("KAFKA_RETRY_MAX")); err == nil && n > 0 {
		cfg.RetryMax = n
	}
	if n, err := strconv.Atoi(os.Getenv("KAFKA_PARTITIONS")); err == nil && n > 0 {
		cfg.Partitions = n
	}
	return cfg
}

type subscription struct {
	queue, topic string
	handler      queue.Handler
}

// Client is a Kafka queue.Queue.
type Client struct {
	cfg    Config
	writer *kafkago.Writer

	mu      sync.Mutex
	topics  map[string]bool
	subs    []subscription
	readers []*kafkago.Reader
	runCtx  context.Context // consumers' lifetime (Start → Stop)
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stats   map[string]*queue.Stats
	started bool
}

// Open checks that a broker is reachable (fails fast so nitro.QueueFromEnv
// can fall back to memory) and prepares the producer.
func Open(cfg Config) (*Client, error) {
	def := DefaultConfig()
	if len(cfg.Brokers) == 0 {
		cfg.Brokers = def.Brokers
	}
	if cfg.GroupID == "" {
		cfg.GroupID = def.GroupID
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = def.RetryMax
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = def.RetryDelay
	}
	if cfg.Partitions <= 0 {
		cfg.Partitions = def.Partitions
	}
	if cfg.ReplicationFactor <= 0 {
		cfg.ReplicationFactor = def.ReplicationFactor
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = def.BatchTimeout
	}
	switch cfg.DLQSuffix {
	case "":
		cfg.DLQSuffix = def.DLQSuffix
	case "-":
		cfg.DLQSuffix = ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := dialAny(ctx, cfg.Brokers)
	if err != nil {
		return nil, fmt.Errorf("kafka: connect: %w", err)
	}
	_ = conn.Close()
	return &Client{
		cfg: cfg,
		writer: &kafkago.Writer{
			Addr:                   kafkago.TCP(cfg.Brokers...),
			Balancer:               &kafkago.Hash{},
			RequiredAcks:           kafkago.RequireAll,
			BatchTimeout:           cfg.BatchTimeout,
			AllowAutoTopicCreation: true,
		},
		topics: make(map[string]bool),
		stats:  make(map[string]*queue.Stats),
	}, nil
}

// OpenEnv is Open(ConfigFromEnv()).
func OpenEnv() (*Client, error) { return Open(ConfigFromEnv()) }

func dialAny(ctx context.Context, brokers []string) (*kafkago.Conn, error) {
	var last error
	for _, b := range brokers {
		conn, err := kafkago.DialContext(ctx, "tcp", b)
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// ensureTopic creates topic on the controller when missing (brokers often run
// with auto.create.topics.enable=false, and consumers need it to exist).
func (c *Client) ensureTopic(ctx context.Context, topic string) error {
	c.mu.Lock()
	ok := c.topics[topic]
	c.mu.Unlock()
	if ok {
		return nil
	}
	conn, err := dialAny(ctx, c.cfg.Brokers)
	if err != nil {
		return fmt.Errorf("kafka: connect: %w", err)
	}
	defer conn.Close()
	ctrl, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("kafka: controller: %w", err)
	}
	cc, err := kafkago.DialContext(ctx, "tcp", net.JoinHostPort(ctrl.Host, strconv.Itoa(ctrl.Port)))
	if err != nil {
		return fmt.Errorf("kafka: controller: %w", err)
	}
	defer cc.Close()
	err = cc.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     c.cfg.Partitions,
		ReplicationFactor: c.cfg.ReplicationFactor,
	})
	if err != nil && !errors.Is(err, kafkago.TopicAlreadyExists) {
		return fmt.Errorf("kafka: create topic %s: %w", topic, err)
	}
	c.mu.Lock()
	c.topics[topic] = true
	c.mu.Unlock()
	return nil
}

// Publish implements queue.Queue — queue is the topic.
func (c *Client) Publish(ctx context.Context, topic string, payload interface{}) error {
	return c.PublishWithOptions(ctx, topic, payload)
}

// PublishWithOptions implements queue.MetaPublisher. Metadata becomes record
// headers; metadata["key"] (e.g. a customer id) is the partition key, keeping
// messages with the same key in order.
func (c *Client) PublishWithOptions(ctx context.Context, topic string, payload interface{}, opts ...queue.PublishOption) error {
	po := queue.ApplyPublishOptions(opts...)
	body, err := marshal(payload)
	if err != nil {
		return err
	}
	if err := c.ensureTopic(ctx, topic); err != nil {
		return err
	}
	maxRetry := c.cfg.RetryMax
	if po.MaxRetry > 0 {
		maxRetry = po.MaxRetry
	}
	id := queue.NewMessageID()
	var key []byte
	headers := make([]kafkago.Header, 0, len(po.Metadata)+2)
	for k, v := range po.Metadata {
		switch k {
		case headerMsgID:
			if v != "" {
				id = v
			}
			continue
		case "key":
			key = []byte(v)
		}
		headers = append(headers, kafkago.Header{Key: k, Value: []byte(v)})
	}
	headers = append(headers,
		kafkago.Header{Key: headerMsgID, Value: []byte(id)},
		kafkago.Header{Key: headerMaxRetry, Value: []byte(strconv.Itoa(maxRetry))},
	)
	if key == nil {
		key = []byte(id)
	}
	if err := c.writer.WriteMessages(ctx, kafkago.Message{Topic: topic, Key: key, Value: body, Headers: headers, Time: time.Now()}); err != nil {
		return fmt.Errorf("kafka: publish: %w", err)
	}
	c.mu.Lock()
	st := c.ensureStats(topic)
	st.Pending++
	st.Total++
	c.mu.Unlock()
	return nil
}

func marshal(payload any) ([]byte, error) {
	if v, ok := payload.(json.RawMessage); ok {
		return v, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("kafka: marshal: %w", err)
	}
	return b, nil
}

// Subscribe implements queue.Queue (topic = queue name).
func (c *Client) Subscribe(name string, handler queue.Handler) error {
	return c.SubscribeRoute(name, name, handler)
}

// SubscribeRoute implements queue.RouteSubscriber: consumer group
// "<GroupID>.<queueName>" reads topic (routingKey).
func (c *Client) SubscribeRoute(queueName, routingKey string, handler queue.Handler) error {
	if handler == nil {
		return errors.New("kafka: nil handler")
	}
	if routingKey == "" {
		routingKey = queueName
	}
	s := subscription{queue: queueName, topic: routingKey, handler: handler}
	c.mu.Lock()
	c.subs = append(c.subs, s)
	started := c.started
	c.mu.Unlock()
	if started {
		return c.consume(s)
	}
	return nil
}

// Start implements queue.Queue.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("kafka: already started")
	}
	c.runCtx, c.cancel = context.WithCancel(ctx)
	c.started = true
	subs := append([]subscription(nil), c.subs...)
	c.mu.Unlock()
	for _, s := range subs {
		if err := c.consume(s); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) consume(s subscription) error {
	ctx := context.Background()
	if err := c.ensureTopic(ctx, s.topic); err != nil {
		return err
	}
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        c.cfg.Brokers,
		GroupID:        c.cfg.GroupID + "." + s.queue,
		Topic:          s.topic,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		MaxWait:        250 * time.Millisecond,
		StartOffset:    kafkago.FirstOffset,
		CommitInterval: 0, // synchronous commit after each handled message
	})
	c.mu.Lock()
	c.readers = append(c.readers, r)
	runCtx := c.runCtx
	c.ensureStats(s.queue)
	c.mu.Unlock()
	c.wg.Add(1)
	go c.loop(runCtx, r, s)
	return nil
}

func (c *Client) loop(ctx context.Context, r *kafkago.Reader, s subscription) {
	defer c.wg.Done()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			log.Printf("[kafka] fetch %s: %v", s.topic, err)
			if !sleepCtx(ctx, c.cfg.RetryDelay) {
				return
			}
			continue
		}
		c.process(ctx, s, m)
		if err := r.CommitMessages(context.Background(), m); err != nil {
			log.Printf("[kafka] commit %s: %v", s.topic, err)
		}
	}
}

// process runs the handler with in-process retries (Kafka has no per-message
// redelivery) and dead-letters the message after RetryMax attempts.
func (c *Client) process(ctx context.Context, s subscription, m kafkago.Message) {
	msg := toMessage(s.queue, m)
	maxRetry := msg.MaxRetry
	if maxRetry <= 0 {
		maxRetry = c.cfg.RetryMax
	}
	c.mu.Lock()
	st := c.ensureStats(s.queue)
	st.Processing++
	c.mu.Unlock()
	var err error
	for attempt := msg.Attempts; attempt < maxRetry; attempt++ {
		msg.Attempts = attempt
		if err = safeHandler(ctx, msg, s.handler); err == nil {
			break
		}
		if attempt+1 < maxRetry && !sleepCtx(ctx, c.cfg.RetryDelay<<attempt) {
			break
		}
	}
	c.mu.Lock()
	st.Processing--
	if err == nil {
		st.Completed++
	} else {
		st.Failed++
	}
	c.mu.Unlock()
	if err == nil || c.cfg.DLQSuffix == "" {
		return
	}
	meta := map[string]string{"x-nitro-error": err.Error(), "x-nitro-origin": s.topic}
	for k, v := range msg.Metadata {
		if _, set := meta[k]; !set {
			meta[k] = v
		}
	}
	if derr := c.PublishWithOptions(context.Background(), s.topic+"."+c.cfg.DLQSuffix, json.RawMessage(msg.Payload), queue.WithMetadata(meta)); derr != nil {
		log.Printf("[kafka] dead-letter %s: %v", s.topic, derr)
	}
}

func toMessage(queueName string, m kafkago.Message) *queue.Message {
	meta := map[string]string{}
	id, maxRetry, attempts := "", 0, 0
	for _, h := range m.Headers {
		v := string(h.Value)
		switch h.Key {
		case headerMsgID:
			id = v
		case headerMaxRetry:
			maxRetry, _ = strconv.Atoi(v)
		case headerAttempts:
			attempts, _ = strconv.Atoi(v)
		default:
			meta[h.Key] = v
		}
	}
	if id == "" {
		id = queue.NewMessageID()
	}
	return &queue.Message{
		ID:        id,
		Queue:     queueName,
		Payload:   append(json.RawMessage(nil), m.Value...),
		State:     queue.StateProcessing,
		Attempts:  attempts,
		MaxRetry:  maxRetry,
		CreatedAt: m.Time,
		UpdatedAt: time.Now().UTC(),
		Metadata:  meta,
	}
}

func safeHandler(ctx context.Context, msg *queue.Message, h queue.Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kafka: handler panic: %v", r)
		}
	}()
	return h(ctx, msg)
}

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

// Stop implements queue.Queue: stops consumers and waits for in-flight handlers.
func (c *Client) Stop() error {
	c.mu.Lock()
	cancel := c.cancel
	readers := c.readers
	c.readers = nil
	c.cancel = nil
	c.started = false
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	var errs []error
	for _, r := range readers {
		if err := r.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close stops consuming and flushes the producer.
func (c *Client) Close() error {
	err := c.Stop()
	if werr := c.writer.Close(); werr != nil {
		err = errors.Join(err, werr)
	}
	return err
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
