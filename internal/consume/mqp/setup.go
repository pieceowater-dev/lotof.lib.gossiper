package gossiper

import (
	"encoding/json"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	"github.com/streadway/amqp"
	"log"
)

type Net struct {
	ConsumerConfig config.AMQPConsumerConfig
}

// DefaultMessage defines the default message structure in Gossiper
type DefaultMessage struct {
	Pattern string      `json:"pattern"`
	Data    interface{} `json:"data"`
}

// DefaultHandleMessage is the default message handler
func DefaultHandleMessage(msg []byte) interface{} {
	var defaultMessage DefaultMessage
	err := json.Unmarshal(msg, &defaultMessage)
	if err != nil {
		log.Println("Failed to unmarshal message:", err)
		return nil
	}
	log.Printf("Received message: %s", defaultMessage.Pattern)
	return "OK"
}

func (n *Net) SetupAMQPConsumers(messageHandler func([]byte) interface{}) {
	if messageHandler == nil {
		messageHandler = DefaultHandleMessage
	}

	env := environment.Env{}
	dsn, err := env.Get(n.ConsumerConfig.DSNEnv)
	if err != nil {
		panic(err)
	}
	conn, err := amqp.Dial(dsn)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to open a channel:", err)
		return
	}
	defer ch.Close()

	for _, consume := range n.ConsumerConfig.Consume {
		msgs, err := ch.Consume(
			consume.Queue, "", consume.AutoAck, consume.Exclusive, consume.NoLocal, consume.NoWait, consume.Args,
		)
		if err != nil {
			log.Fatal("Failed to register a consumer:", err)
			return
		}

		go func() {
			for d := range msgs {
				response := messageHandler(d.Body)

				if d.ReplyTo != "" {
					responseBytes, _ := json.Marshal(response)
					err = ch.Publish("", d.ReplyTo, false, false, amqp.Publishing{
						ContentType:   "application/json",
						CorrelationId: d.CorrelationId,
						Body:          responseBytes,
					})

					if err != nil {
						log.Println("Failed to publish response:", err)
					}
				}
			}
		}()
	}

	log.Println("Waiting for messages. To exit press CTRL+C")
	select {}
}
