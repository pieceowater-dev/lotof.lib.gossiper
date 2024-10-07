package conf

import "github.com/rabbitmq/amqp091-go"

// Config holds the overall configuration for the gossiper package.
// It includes both the environment variable management (Env), RabbitMQ consumer configuration (AMQPConsumer), and Database settings (Database).
type Config struct {
	Env          EnvConfig          // Environment variable settings (required for both RabbitMQ and Database)
	AMQPConsumer AMQPConsumerConfig // RabbitMQ consumer settings
	Database     DatabaseConfig     // Database configuration settings
}

// DatabaseConfig defines the structure for database-related configurations.
type DatabaseConfig struct {
	PG         DBPGConfig         // PG-specific configuration
	ClickHouse DBClickHouseConfig // ClickHouse-specific configuration
}

// DBPGConfig holds PostgreSQL-related configuration options.
type DBPGConfig struct {
	EnvPostgresDBDSN string // Environment variable for PostgreSQL DSN
	AutoMigrate      bool   // Whether to automatically run migrations
	Models           []any  // List of models for auto-migration
}

type DBClickHouseConfig struct {
	EnvClickHouseDBDSN string // Environment variable for ClickHouse DSN
	AutoMigrate        bool   // Whether to automatically run migrations
	Models             []any  // List of models for auto-migration
}

// EnvConfig defines the required environment variables needed by the application.
type EnvConfig struct {
	Required []string // List of required environment variables for proper application functioning
}

// Validate checks if all required environment variables are present.
// Returns nil for now, but can be extended to perform actual validation.
func (ec *EnvConfig) Validate() error {
	return nil // Validation logic can be added here to ensure all required variables are set
}

// QueueConfig defines the configuration for RabbitMQ queues.
type QueueConfig struct {
	Name       string        // Name of the RabbitMQ queue to declare
	Durable    bool          // If true, the queue persists across broker restarts
	AutoDelete bool          // If true, the queue is deleted when it's no longer used
	Exclusive  bool          // If true, the queue is exclusive to the connection that declared it
	NoWait     bool          // If true, the server doesn't wait for confirmation after declaring the queue
	Args       amqp091.Table // Custom arguments for queue declaration (advanced settings)
}

// AMQPConsumerConfig holds the configuration for RabbitMQ consumers.
// It defines how RabbitMQ queues are consumed and what settings to apply for each consumer.
type AMQPConsumerConfig struct {
	DSNEnv  string              // The environment variable that holds the RabbitMQ DSN
	Queues  []QueueConfig       // List of queues to declare in RabbitMQ
	Consume []AMQPConsumeConfig // List of consumers and their consumption settings
}

// AMQPConsumeConfig defines the settings for consuming messages from a queue.
// Each consumer has specific settings such as auto-acknowledgment, exclusivity, etc.
type AMQPConsumeConfig struct {
	Queue     string        // Name of the queue to consume messages from
	Consumer  string        // Consumer tag that identifies this specific consumer
	AutoAck   bool          // If true, the consumer automatically acknowledges messages after receiving them
	Exclusive bool          // If true, only this consumer can access the queue
	NoLocal   bool          // If true, prevents messages published on the same connection from being consumed
	NoWait    bool          // If true, the server doesn't wait for confirmation after setting up the consumer
	Args      amqp091.Table // Custom arguments for consumer setup (advanced settings)
}

// Validate checks if the AMQP consumer configuration is valid.
// Currently, returns nil, but this function can be extended to validate the consumer setup.
func (acc *AMQPConsumerConfig) Validate() error {
	return nil // Future validation logic to ensure correct consumer configurations can go here
}
