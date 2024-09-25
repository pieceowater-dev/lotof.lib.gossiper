package gossiper

import "github.com/streadway/amqp"

type Config struct {
	Env          EnvConfig
	AMQPConsumer AMQPConsumerConfig
}

type EnvConfig struct {
	Required []string
}

func (ec *EnvConfig) Validate() error {
	return nil
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

func (acc *AMQPConsumerConfig) Validate() error {
	return nil
}
