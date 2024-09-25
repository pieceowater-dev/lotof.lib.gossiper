package config

import "github.com/streadway/amqp"

// Config - Custom Configuration structure
type Config struct {
	Env          EnvConfig
	AMQPConsumer ConsumerConfig
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

type ConsumerConfig struct {
	Queues  []QueueConfig
	Consume []ConsumeConfig
}

type ConsumeConfig struct {
	Queue     string
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}
