package amqp

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client struct to manage AMQP connections, channels, queues, etc.
type Client struct {
	QueueName string
	DSN       string
	conn      *amqp.Connection
	channel   *amqp.Channel
}

// New creates a new AMQP client
func New(queueName, dsn string) (*Client, error) {
	client := &Client{
		QueueName: queueName,
		DSN:       dsn,
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes a connection to RabbitMQ
func (c *Client) connect() error {
	conn, err := amqp.Dial(c.DSN)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	c.conn = conn
	c.channel = ch

	return nil
}

// Close closes the connection and channel
func (c *Client) Close() {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// SendMessage sends a message to the queue with retry logic and optional reply
func (c *Client) SendMessage(body []byte, reply bool) ([]byte, error) {
	var err error
	for i := 0; i < 3; i++ {
		if reply {
			response, err := c.sendWithReply(body)
			if err == nil {
				return response, nil
			}
		} else {
			err = c.send(body)
			if err == nil {
				return nil, nil
			}
		}
		log.Printf("Error sending message, retrying... Attempt %d/3", i+1)
		time.Sleep(3 * time.Second)
	}
	return nil, err
}

// send sends a message without waiting for a reply
func (c *Client) send(body []byte) error {
	return c.channel.PublishWithContext(
		context.TODO(),
		"", // exchange
		c.QueueName,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// sendWithReply sends a message and waits for a reply
func (c *Client) sendWithReply(body []byte) ([]byte, error) {
	// Declare an anonymous queue for replies
	q, err := c.channel.QueueDeclare(
		"",    // name
		false, // durable
		true,  // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare reply queue: %w", err)
	}

	//corrID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Set up the message publishing with a reply-to header
	err = c.channel.PublishWithContext(
		context.TODO(),
		"", // exchange
		c.QueueName,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			Body:          body,
			ReplyTo:       q.Name,
			CorrelationId: fmt.Sprintf("%d", time.Now().UnixNano()), // adjust as needed
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// Consume the reply message
	msgs, err := c.channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // autoAck
		false,  // exclusive
		false,  // noLocal
		false,  // noWait
		nil,    // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to consume reply: %w", err)
	}

	// Wait for a single response
	for msg := range msgs {
		return msg.Body, nil
	}

	return nil, fmt.Errorf("no reply received")
}

// Consume starts consuming messages from the queue
func (c *Client) Consume(handler func([]byte)) error {
	msgs, err := c.channel.Consume(
		c.QueueName,
		"",
		true,  // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	go func() {
		for d := range msgs {
			handler(d.Body)
		}
	}()

	return nil
}
