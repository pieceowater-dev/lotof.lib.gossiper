package main

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"log"
)

func main() {
	conf := gossiper.Config{
		Env: gossiper.EnvConfig{
			Required: []string{"RABBITMQ_DSN"},
		},
		AMQPConsumer: gossiper.AMQPConsumerConfig{
			Queues: []gossiper.QueueConfig{
				{
					Name:       "template_queue",
					Durable:    true,
					AutoDelete: false,
					Exclusive:  false,
					NoWait:     false,
					Args:       nil,
				},
			},
			Consume: []gossiper.AMQPConsumeConfig{
				{
					Queue:     "template_queue",
					Consumer:  "example_consumer",
					AutoAck:   true,
					Exclusive: false,
					NoLocal:   false,
					NoWait:    false,
					Args:      nil,
				},
			},
		},
	}
	gossiper.Setup(conf)
	// Application logic
	log.Println("Application started")
}
