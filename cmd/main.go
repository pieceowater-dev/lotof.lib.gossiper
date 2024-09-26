package main

import (
	"encoding/json"
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"log"
)

func HandleMessage(msg gossiper.AMQMessage) interface{} {
	// some routing logic here
	log.Printf("Received message: %s", msg.Pattern)
	return "OK"
}

func main() {
	conf := gossiper.Config{
		Env: gossiper.EnvConfig{
			Required: []string{"RABBITMQ_DSN"},
		},
		AMQPConsumer: gossiper.AMQPConsumerConfig{
			DSNEnv: "RABBITMQ_DSN",
			Queues: []gossiper.QueueConfig{
				{
					Name:    "template_queue",
					Durable: true,
				},
			},
			Consume: []gossiper.AMQPConsumeConfig{
				{
					Queue:    "template_queue",
					Consumer: "example_consumer",
					AutoAck:  true,
				},
			},
		},
	}

	//gossiper.Setup(conf, nil)
	gossiper.Setup(conf, func(msg []byte) interface{} {
		var customMessage gossiper.AMQMessage
		err := json.Unmarshal(msg, &customMessage)
		if err != nil {
			log.Println("Failed to unmarshal custom message:", err)
			return nil
		}
		return HandleMessage(customMessage)
	})

	log.Println("Application started")
}
