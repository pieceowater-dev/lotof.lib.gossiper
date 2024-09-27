package gossiper

import "github.com/rabbitmq/amqp091-go"

// Config holds the overall configuration for the gossiper package.
// It includes both the environment variable management (Env) and the RabbitMQ consumer configuration (AMQPConsumer).
type Config struct {
	Env          EnvConfig          // Environment variable settings
	AMQPConsumer AMQPConsumerConfig // RabbitMQ consumer settings
}

// EnvConfig defines the required environment variables needed by the application.
type EnvConfig struct {
	Required []string // List of required environment variables
}

// Validate checks if all required environment variables are present.
// Returns nil for now, but can be extended to perform actual validation.
func (ec *EnvConfig) Validate() error {
	return nil
}

// QueueConfig defines the configuration for RabbitMQ queues.
type QueueConfig struct {
	Name       string        // Name of the RabbitMQ queue
	Durable    bool          // If true, the queue survives broker restarts
	AutoDelete bool          // If true, the queue is automatically deleted when no longer in use
	Exclusive  bool          // If true, the queue is used only by the connection that declared it
	NoWait     bool          // If true, the server doesn't wait for a confirmation after declaring the queue
	Args       amqp091.Table // Custom arguments for queue declaration
}

// AMQPConsumerConfig holds the configuration for RabbitMQ consumers.
type AMQPConsumerConfig struct {
	DSNEnv  string              // The environment variable name for the RabbitMQ DSN
	Queues  []QueueConfig       // List of queues to declare in RabbitMQ
	Consume []AMQPConsumeConfig // List of consumers and their settings for message consumption
}

// AMQPConsumeConfig defines the settings for consuming messages from a queue.
type AMQPConsumeConfig struct {
	Queue     string        // Name of the queue to consume from
	Consumer  string        // Consumer tag to identify the consumer
	AutoAck   bool          // If true, messages are automatically acknowledged after being received
	Exclusive bool          // If true, the queue is consumed by only this consumer
	NoLocal   bool          // If true, messages published on the same connection will not be received by this consumer
	NoWait    bool          // If true, the server doesn't wait for a confirmation after setting up the consumer
	Args      amqp091.Table // Custom arguments for consumer setup
}

// Validate checks if the consumer configuration is valid.
// Currently, returns nil, but this function can be extended to ensure proper validation.
func (acc *AMQPConsumerConfig) Validate() error {
	return nil
}
