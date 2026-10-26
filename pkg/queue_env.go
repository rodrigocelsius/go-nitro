package nitro

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/queue/v2"
)

// FallbackQueue is a broker queue that degrades to memory (see QueueFromEnv).
type FallbackQueue = queue.Fallback

// FallbackQueueConfig tunes FallbackQueue.
type FallbackQueueConfig = queue.FallbackConfig

// ErrQueueFull is returned when an in-memory buffer stays full.
var ErrQueueFull = queue.ErrQueueFull

// NewFallbackQueue wraps primary (e.g. a *RabbitQueue) so publishing keeps
// working while it is down: subscribed queues are processed in memory, the
// rest is buffered and re-published when the broker is back. nil primary →
// in-memory only.
func NewFallbackQueue(primary Queue, cfg ...FallbackQueueConfig) *FallbackQueue {
	var c FallbackQueueConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return queue.NewFallback(primary, c)
}

// brokerEntry is a broker QueueFromEnv can select from the environment.
type brokerEntry struct {
	name, env string
	open      func() (Queue, error)
}

// brokers in selection order (RabbitMQ first; NATS / Kafka register from
// build-tagged files so they can be compiled out).
var brokers = []brokerEntry{
	{name: "rabbitmq", env: "RABBITMQ_URL", open: func() (Queue, error) { return NewRabbitQueueFromEnv() }},
}

func registerBroker(name, env string, open func() (Queue, error)) {
	brokers = append(brokers, brokerEntry{name: name, env: env, open: open})
}

// selectBroker picks NITRO_QUEUE (rabbitmq|nats|kafka|memory) when set, else
// the first broker whose URL variable is set; nil means in-memory.
func selectBroker() *brokerEntry {
	if want := strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_QUEUE"))); want != "" {
		if want == "memory" {
			return nil
		}
		for i := range brokers {
			if brokers[i].name == want {
				return &brokers[i]
			}
		}
		log.Printf("[queue] NITRO_QUEUE=%q is not compiled in (built with -tags nitro_no%s?); using memory", want, want)
		return nil
	}
	for _, name := range []string{"rabbitmq", "nats", "kafka"} {
		for i := range brokers {
			if brokers[i].name == name && os.Getenv(brokers[i].env) != "" {
				return &brokers[i]
			}
		}
	}
	return nil
}

// QueueFromEnv is the queue convention — no branching in main:
//
//   - RABBITMQ_URL → RabbitMQ (RABBITMQ_EXCHANGE / RABBITMQ_PREFETCH)
//   - NATS_URL → NATS JetStream (streams / consumers created on first use)
//   - KAFKA_BROKERS → Kafka (topics / consumer groups created on first use)
//   - none → in-process queue (workers in this process)
//
// NITRO_QUEUE=rabbitmq|nats|kafka|memory picks one explicitly. Brokers are
// wrapped in FallbackQueue: while the broker is down (at boot or later),
// messages are processed in memory (local subscribers) or buffered and
// re-sent when it is back; the connection is retried in the background.
//
//	q := nitro.QueueFromEnv()
//	_ = q.Subscribe("emails", sendEmail)
//	_ = q.Start(ctx)
//	defer q.Stop()
func QueueFromEnv() *FallbackQueue {
	b := selectBroker()
	if b == nil {
		return queue.NewFallback(nil, queue.FallbackConfig{})
	}
	return queue.NewFallback(&lazyBroker{name: b.name, open: b.open}, queue.FallbackConfig{})
}

// lazyBroker dials the broker on first use and again after a failed dial
// (at most every redialEvery), replaying subscriptions once connected.
type lazyBroker struct {
	name string
	open func() (Queue, error)

	mu       sync.Mutex
	q        Queue
	lastTry  time.Time
	subs     []lazySub
	startCtx context.Context
	warned   bool
}

type lazySub struct {
	queue, key string
	h          QueueHandler
}

const redialEvery = 5 * time.Second

func (l *lazyBroker) get() (Queue, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.q != nil {
		return l.q, nil
	}
	if !l.lastTry.IsZero() && time.Since(l.lastTry) < redialEvery {
		return nil, errBrokerDown
	}
	l.lastTry = time.Now()
	q, err := l.open()
	if err != nil {
		if !l.warned {
			l.warned = true
			log.Printf("[queue] %s unreachable (%v); using in-memory fallback, retrying every %v", l.name, err, redialEvery)
		}
		return nil, err
	}
	for _, s := range l.subs {
		if err := SubscribeRoute(q, s.queue, s.key, s.h); err != nil {
			log.Printf("[queue] resubscribe %q: %v", s.queue, err)
		}
	}
	if l.startCtx != nil {
		if err := q.Start(l.startCtx); err != nil {
			log.Printf("[queue] %s start: %v", l.name, err)
		}
	}
	l.q = q
	l.warned = false
	log.Printf("[queue] %s connected", l.name)
	return q, nil
}

var errBrokerDown = queueError("queue: broker unavailable")

type queueError string

func (e queueError) Error() string { return string(e) }

func (l *lazyBroker) Publish(ctx context.Context, key string, payload any) error {
	q, err := l.get()
	if err != nil {
		return err
	}
	return q.Publish(ctx, key, payload)
}

func (l *lazyBroker) PublishWithOptions(ctx context.Context, key string, payload any, opts ...PublishOption) error {
	q, err := l.get()
	if err != nil {
		return err
	}
	return PublishWithOptions(ctx, q, key, payload, opts...)
}

func (l *lazyBroker) Subscribe(name string, h QueueHandler) error {
	return l.SubscribeRoute(name, name, h)
}

func (l *lazyBroker) SubscribeRoute(name, key string, h QueueHandler) error {
	l.mu.Lock()
	l.subs = append(l.subs, lazySub{queue: name, key: key, h: h})
	q := l.q
	l.mu.Unlock()
	if q != nil {
		return SubscribeRoute(q, name, key, h)
	}
	return nil
}

// Start connects (or keeps retrying in the background) and starts consumers.
func (l *lazyBroker) Start(ctx context.Context) error {
	l.mu.Lock()
	l.startCtx = ctx
	l.mu.Unlock()
	if _, err := l.get(); err != nil {
		go l.redialLoop(ctx)
	}
	return nil
}

func (l *lazyBroker) redialLoop(ctx context.Context) {
	t := time.NewTicker(redialEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := l.get(); err == nil {
				return
			}
		}
	}
}

func (l *lazyBroker) Stop() error {
	l.mu.Lock()
	q := l.q
	l.mu.Unlock()
	if q == nil {
		return nil
	}
	if c, ok := q.(interface{ Close() error }); ok {
		return c.Close()
	}
	return q.Stop()
}

func (l *lazyBroker) Stats(ctx context.Context, name string) (*QueueStats, error) {
	l.mu.Lock()
	q := l.q
	l.mu.Unlock()
	if q == nil {
		return &QueueStats{}, nil
	}
	return q.Stats(ctx, name)
}
