package amqp

import (
	"encoding/json"
	t "github.com/pieceowater-dev/lotof.lib.gossiper/types"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
)

// DefaultHandleMessage is the default message handler used by Gossiper.
// It unmarshals the message into a DefaultMessage structure and logs the pattern.
func DefaultHandleMessage(msg []byte) any {
	var defaultMessage t.DefaultMessage
	if err := json.Unmarshal(msg, &defaultMessage); err != nil {
		log.Println("Failed to unmarshal message:", err)
		return nil
	}
	log.Printf("Received message: %s", defaultMessage.Pattern)
	return "OK" // Default response; modify based on message type
}

// handleMessages processes incoming messages asynchronously.
func (n *AMQP) handleMessages(msgs <-chan amqp.Delivery, ch *amqp.Channel, messageHandler func([]byte) any) {
	for d := range msgs {
		response := messageHandler(d.Body)

		if d.ReplyTo != "" {
			if err := n.publishResponse(ch, d.ReplyTo, d.CorrelationId, response); err != nil {
				log.Println("Failed to publish response:", err)
			}
		}
	}
}

// handleError checks the error and logs it with a custom message.
func handleError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}
