//go:build !nitro_nokafka

package nitro

import kafkaadapter "github.com/rodrigocelsius/go-nitro/adapters/kafka/v2"

// Kafka / Redpanda broker. Exclude with -tags nitro_nokafka.

// KafkaConfig configures the Kafka queue.
type KafkaConfig = kafkaadapter.Config

// KafkaQueue is the Kafka client (implements Queue).
type KafkaQueue = kafkaadapter.Client

// NewKafkaQueue connects to Kafka. Topics are created on first use and every
// Subscribe gets its own consumer group — no broker-side setup.
func NewKafkaQueue(cfg KafkaConfig) (*KafkaQueue, error) { return kafkaadapter.Open(cfg) }

// NewKafkaQueueFromEnv reads KAFKA_BROKERS / KAFKA_GROUP_ID / KAFKA_RETRY_MAX / KAFKA_PARTITIONS.
func NewKafkaQueueFromEnv() (*KafkaQueue, error) { return kafkaadapter.OpenEnv() }

func init() {
	registerBroker("kafka", "KAFKA_BROKERS", func() (Queue, error) { return kafkaadapter.OpenEnv() })
}
