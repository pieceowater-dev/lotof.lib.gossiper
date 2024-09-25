package main

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"log"
)

// example of usage
func main() {
	conf := gossiper.Cfg{
		Env: gossiper.EnvCfg{
			Required: []string{"RABBITMQ_DSN"},
		},
		AMQPConsumer: gossiper.AMQPConsumerCfg{
			Queues: []gossiper.QueueCfg{
				{
					Name:       "template_queue",
					Durable:    true,
					AutoDelete: false,
					Exclusive:  false,
					NoWait:     false,
					Args:       nil,
				},
			},
			Consume: []gossiper.AMQPConsumeCfg{
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
