package main

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"github.com/pieceowater-dev/lotof.lib.gossiper/config"
	"log"
)

// example of usage
func main() {
	conf := config.Config{
		Env: config.EnvConfig{
			Required: []string{"RABBITMQ_DSN"},
		},
		AMQPConsumer: config.ConsumerConfig{
			Queues: []config.QueueConfig{
				{
					Name:       "template_queue",
					Durable:    true,
					AutoDelete: false,
					Exclusive:  false,
					NoWait:     false,
					Args:       nil,
				},
			},
			Consume: []config.ConsumeConfig{
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
	// Initialize gossiper with configuration
	gossiper.Setup(conf)

	// Application logic here
	log.Println("Application started")
}
