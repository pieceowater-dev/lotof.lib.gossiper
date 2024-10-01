package amqp

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"strings"
)

// AMQP holds the RabbitMQ consumer configuration
type AMQP struct {
	ConsumerConfig conf.AMQPConsumerConfig // Configuration for RabbitMQ consumers
}

// SetupAMQPConsumers initializes and starts RabbitMQ consumers based on the configuration in AMQP.
// It processes incoming messages and uses the provided messageHandler to handle them.
func (n *AMQP) SetupAMQPConsumers(messageHandler func([]byte) any) {
	if messageHandler == nil {
		messageHandler = DefaultHandleMessage
	}

	// Load RabbitMQ DSN from environment variables
	envInst := env.Env{}
	dsn, err := envInst.Get(n.ConsumerConfig.DSNEnv)
	if err != nil {
		log.Fatalf("Error loading DSN: %v", err)
	}

	// Establish a connection to RabbitMQ
	conn, err := amqp.Dial(dsn)
	handleError(err, "Failed to connect to RabbitMQ")
	defer conn.Close() // Ensure connection is closed when done

	// Open a channel over the connection
	ch, err := conn.Channel()
	handleError(err, "Failed to open a channel")
	defer ch.Close() // Ensure channel is closed

	var queueNames []string

	// Declare queues first from QueueConfig
	for _, queueConfig := range n.ConsumerConfig.Queues {
		if err := declareQueue(ch, queueConfig); err != nil {
			log.Fatalf("Failed to declare queue: %v", err)
		}
		queueNames = append(queueNames, queueConfig.Name)
	}

	// Now set up the consumers from AMQPConsumeConfig
	for _, consumeConfig := range n.ConsumerConfig.Consume {
		msgs, err := ch.Consume(
			consumeConfig.Queue,
			consumeConfig.Consumer,
			consumeConfig.AutoAck,
			consumeConfig.Exclusive,
			consumeConfig.NoLocal,
			consumeConfig.NoWait,
			consumeConfig.Args,
		)
		handleError(err, "Failed to register a consumer")

		// Start a goroutine to handle messages asynchronously
		go n.handleMessages(msgs, ch, messageHandler)
	}

	// Log that the consumer setup is complete and the service is ready for messages
	log.Printf("Service successfully started! [%s]", " ⇆"+strings.Join(queueNames, " ⇆"))

	// Block the function indefinitely, waiting for messages
	select {}
}

// declareQueue declares a RabbitMQ queue if it doesn't already exist.
func declareQueue(ch *amqp.Channel, queueConfig conf.QueueConfig) error {
	_, err := ch.QueueDeclare(
		queueConfig.Name,
		queueConfig.Durable,
		queueConfig.AutoDelete,
		queueConfig.Exclusive,
		queueConfig.NoWait,
		queueConfig.Args, // Additional arguments for queue declaration
	)
	return err
}
