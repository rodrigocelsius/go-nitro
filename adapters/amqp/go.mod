module github.com/rodrigocelsius/go-nitro/adapters/amqp/v2

go 1.22.2

require (
	github.com/rabbitmq/amqp091-go v1.16.0
	github.com/rodrigocelsius/go-nitro/queue/v2 v2.2.0
)

require github.com/google/uuid v1.6.0 // indirect

replace github.com/rodrigocelsius/go-nitro/queue/v2 => ../../queue
