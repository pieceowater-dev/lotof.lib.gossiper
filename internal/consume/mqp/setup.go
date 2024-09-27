package gossiper

import (
	"encoding/json"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
)

// AMQP holds the RabbitMQ consumer configuration
type AMQP struct {
	ConsumerConfig config.AMQPConsumerConfig // Configuration for RabbitMQ consumers
}

// DefaultMessage defines the default message structure for Gossiper
// Pattern is the type or category of the message, Data is the payload
type DefaultMessage struct {
	Pattern string      `json:"pattern"`
	Data    interface{} `json:"data"`
}

// DefaultHandleMessage is the default message handler used by Gossiper.
// It unmarshals the message into a DefaultMessage structure and logs the pattern.
func DefaultHandleMessage(msg []byte) interface{} {
	var defaultMessage DefaultMessage
	err := json.Unmarshal(msg, &defaultMessage)
	if err != nil {
		log.Println("Failed to unmarshal message:", err)
		return nil
	}
	log.Printf("Received message: %s", defaultMessage.Pattern)
	return "OK" // Default response; modify based on message type
}

// SetupAMQPConsumers initializes and starts RabbitMQ consumers based on the configuration in AMQP.
// It processes incoming messages and uses the provided messageHandler to handle them.
func (n *AMQP) SetupAMQPConsumers(messageHandler func([]byte) interface{}) {
	// Use the default handler if no custom handler is provided
	if messageHandler == nil {
		messageHandler = DefaultHandleMessage
	}

	// Load RabbitMQ DSN from environment variables
	env := environment.Env{}
	dsn, err := env.Get(n.ConsumerConfig.DSNEnv)
	if err != nil {
		panic(err) // Panic if the DSN is missing or invalid
	}

	// Establish a connection to RabbitMQ
	conn, err := amqp.Dial(dsn)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close() // Ensure connection is closed when done

	// Open a channel over the connection
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to open a channel:", err)
		return
	}
	defer ch.Close() // Ensure channel is closed

	// Loop over the configured consumers and set them up
	for _, consume := range n.ConsumerConfig.Consume {
		// Start consuming messages from the specified queue
		msgs, err := ch.Consume(
			consume.Queue,     // Queue name
			"",                // Consumer tag (empty means auto-generated)
			consume.AutoAck,   // Automatic message acknowledgment
			consume.Exclusive, // Exclusive consumption by this consumer
			consume.NoLocal,   // Prevent consuming messages published on the same connection
			consume.NoWait,    // Do not wait for a server response
			consume.Args,      // Additional arguments
		)
		if err != nil {
			log.Fatal("Failed to register a consumer:", err)
			return
		}

		// Start a goroutine to handle messages asynchronously
		go func() {
			for d := range msgs {
				// Process each message using the handler
				response := messageHandler(d.Body)

				// If the message has a ReplyTo address, send a response
				if d.ReplyTo != "" {
					responseBytes, _ := json.Marshal(response) // Marshal the response into JSON

					// Publish the response to the reply queue
					err = ch.Publish(
						"",        // Exchange (empty string for default)
						d.ReplyTo, // Routing key (use the ReplyTo property)
						false,     // Mandatory flag
						false,     // Immediate flag
						amqp.Publishing{
							ContentType:   "application/json", // Set content type to JSON
							CorrelationId: d.CorrelationId,    // Maintain correlation ID
							Body:          responseBytes,      // Set the body to the marshaled response
						},
					)

					// Log any error during response publication
					if err != nil {
						log.Println("Failed to publish response:", err)
					}
				}
			}
		}()
	}

	// Log that the consumer setup is complete and the service is ready for messages
	log.Println("Waiting for messages. To exit press CTRL+C")
	// Block the function indefinitely, waiting for messages
	select {}
}
