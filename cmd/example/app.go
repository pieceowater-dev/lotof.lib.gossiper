package main

import (
	"encoding/json"
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"log"
)

// HandleMessage processes incoming RabbitMQ messages.
// It receives an AMQMessage, logs the pattern, and returns a response.
// This is where you can add custom logic to route or process messages.
func HandleMessage(msg gossiper.AMQMessage) any {
	// Log the received message's pattern
	log.Printf("Received message: %s", msg.Pattern)
	return "OK" // Return a response; modify this as needed
}

func main() {
	// Define the gossiper configuration
	conf := gossiper.Config{
		Env: gossiper.EnvConfig{
			Required: []string{"RABBITMQ_DSN"}, // Specify required environment variables
		},
		AMQPConsumer: gossiper.AMQPConsumerConfig{
			DSNEnv: "RABBITMQ_DSN", // RabbitMQ DSN pulled from environment variables
			Queues: []gossiper.QueueConfig{
				{
					Name:    "template_queue", // Queue name
					Durable: true,             // Persistent queue that survives restarts
				},
			},
			Consume: []gossiper.AMQPConsumeConfig{
				{
					Queue:    "template_queue",   // Queue to consume from
					Consumer: "example_consumer", // Consumer tag
					AutoAck:  true,               // Automatically acknowledge messages
				},
			},
		},
	}

	// Initialize and start consuming messages
	// Pass a handler function to process each message
	gossiper.Setup(
		conf,
		nil,
		func(msg []byte) any {
			var customMessage gossiper.AMQMessage

			// Attempt to unmarshal the received message into a custom structure
			err := json.Unmarshal(msg, &customMessage)
			if err != nil {
				log.Println("Failed to unmarshal custom message:", err)
				return nil // Return nil in case of unmarshalling failure
			}

			// Delegate message processing to the HandleMessage function
			return HandleMessage(customMessage)
		},
	)

	// Log that the application has started successfully
	log.Println("Application started")
}
