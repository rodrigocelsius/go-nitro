//go:build !nitro_nonats

package nitro

import natsadapter "github.com/rodrigocelsius/go-nitro/adapters/nats/v2"

// NATS (JetStream) broker. Exclude with -tags nitro_nonats.

// NATSConfig configures the NATS JetStream queue.
type NATSConfig = natsadapter.Config

// NATSQueue is the NATS JetStream client (implements Queue).
type NATSQueue = natsadapter.Client

// NewNATSQueue connects to NATS JetStream. Streams and durable consumers are
// created on first use — no server-side setup.
func NewNATSQueue(cfg NATSConfig) (*NATSQueue, error) { return natsadapter.Open(cfg) }

// NewNATSQueueFromEnv reads NATS_URL / NATS_RETRY_MAX / NATS_PREFETCH.
func NewNATSQueueFromEnv() (*NATSQueue, error) { return natsadapter.OpenEnv() }

func init() {
	registerBroker("nats", "NATS_URL", func() (Queue, error) { return natsadapter.OpenEnv() })
}
