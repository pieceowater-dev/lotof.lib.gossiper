package amqp

import (
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
)

// publishResponse publishes a response to the reply queue.
func (n *AMQP) publishResponse(ch *amqp.Channel, replyTo string, correlationID string, response any) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return ch.Publish(
		"",
		replyTo,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: correlationID,
			Body:          responseBytes,
		},
	)
}
