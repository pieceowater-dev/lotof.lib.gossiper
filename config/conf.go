package config

import "github.com/streadway/amqp"

// Config - Custom Configuration structure
type Config struct {
	Env          EnvConfig
	AMQPConsumer AMQPConsumerConfig
}

type EnvConfig struct {
	Required []string
}

type QueueConfig struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}

type AMQPConsumerConfig struct {
	Queues  []QueueConfig
	Consume []AMQPConsumeConfig
}

type AMQPConsumeConfig struct {
	Queue     string
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}
