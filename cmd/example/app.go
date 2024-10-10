package main

import (
	"encoding/json"
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"gorm.io/gorm"
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

//type SomeData struct {
//	gorm.Model
//	ID   int             `json:"id"`
//	Data json.RawMessage `json:"data"`
//}

func main() {
	// Define the Gossiper configuration for RabbitMQ and PostgreSQL
	conf := gossiper.Config{
		Env: gossiper.EnvConfig{
			Required: []string{"RABBITMQ_DSN", "DATABASE_DSN"}, // Specify required environment variables for RabbitMQ and PostgreSQL
		},
		AMQPConsumer: gossiper.AMQPConsumerConfig{
			DSNEnv: "RABBITMQ_DSN", // Environment variable for RabbitMQ DSN
			Queues: []gossiper.QueueConfig{
				{
					Name:    "template_queue", // Queue name from which messages will be consumed
					Durable: true,             // Set queue as persistent (survives RabbitMQ restarts)
				},
			},
			Consume: []gossiper.AMQPConsumeConfig{
				{
					Queue:    "template_queue",   // Queue name to consume from
					Consumer: "example_consumer", // Unique consumer tag for the connection
					AutoAck:  true,               // Automatically acknowledge receipt of messages
				},
			},
		},
		Database: gossiper.DatabaseConfig{
			PG: gossiper.DBPGConfig{
				EnvPostgresDBDSN: "DATABASE_DSN", // Environment variable key for PostgreSQL DSN
				AutoMigrate:      true,           // Enable auto-migration of models
				Models:           []any{
					// Your models go here
					// &yourModel{}, // Example: Define the models that will be auto-migrated
				},
			},
			ClickHouse: gossiper.DBClickHouseConfig{
				EnvClickHouseDBDSN: "CLICKHOUSE_DSN",
				AutoMigrate:        true,
				Models:             []any{
					//&SomeData{},
				},
				GORMConfig: &gorm.Config{},
			},
		},
	}

	// Initialize the Gossiper application and setup RabbitMQ consumers and PostgreSQL connection
	// Pass a handler function to process each message that is consumed
	app := gossiper.Bootstrap{}
	app.Setup(
		conf,
		func() any {
			// Custom startup logic to execute after initialization (if needed)
			log.Println("Custom Setup here")
			return nil
		},
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
