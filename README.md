# Gossiper

`gossiper` is a lightweight Go package designed to simplify working with environment variables, RabbitMQ, and validation tools. It streamlines the process of managing configurations and consuming messages from RabbitMQ dynamically.

## Installation

```bash
go get github.com/pieceowater-dev/lotof.lib.gossiper
```

## Usage

Instead of placing the entire configuration inside `main.go`, it's better to separate the configuration into its own file and import it in `main.go`. This helps keep the code organized, especially as the application grows.

### Step 1: Create a Config File

Create a file named `config.go` where you can define the configuration for the environment variables and RabbitMQ consumers.

```go
package main

import "github.com/pieceowater-dev/lotof.lib.gossiper"

func GetConfig() gossiper.Config {
	return gossiper.Config{
		Env: gossiper.EnvConfig{
			Required: []string{"RABBITMQ_DSN"},
		},
		AMQPConsumer: gossiper.AMQPConsumerConfig{
			DSNEnv: "RABBITMQ_DSN",
			Queues: []gossiper.QueueConfig{
				{
					Name:       "template_queue",
					Durable:    true,
					AutoDelete: false,
					Exclusive:  false,
					NoWait:     false,
					Args:       nil,
				},
			},
			Consume: []gossiper.AMQPConsumeConfig{
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
}
```

### Step 2: Import Config into `main.go`

Now, in your `main.go`, import the configuration and use it in the `gossiper.Setup` function.

```go
package main

import (
	"encoding/json"
	"github.com/pieceowater-dev/lotof.lib.gossiper"
	"log"
)

func HandleMessage(msg gossiper.AMQMessage) any {
	log.Printf("Received message: %s", msg.Pattern)
	return "OK"
}

func main() {
	// Import the configuration from config.go
	conf := GetConfig()

	// Initialize and start the consumers
	gossiper.Setup(conf, func(msg []byte) any {
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
```

## Configuration

### `gossiper.Config`
This struct is the core configuration and includes:

- **EnvConfig**: Manages environment variables.
- **AMQPConsumerConfig**: Configures RabbitMQ consumers.

### `EnvConfig`

```go
type EnvConfig struct {
    Required []string
}
```

- **Required** (`[]string`): Specifies a list of environment variables that are mandatory for the application to run. If any variable is missing, the application will return an error.

### `AMQPConsumerConfig`

```go
type AMQPConsumerConfig struct {
    DSNEnv  string
    Queues  []QueueConfig
    Consume []AMQPConsumeConfig
}
```

- **DSNEnv** (`string`): The environment variable name that stores the RabbitMQ DSN (Data Source Name). This value is retrieved from the environment.
- **Queues** (`[]QueueConfig`): A list of queues that should be declared in RabbitMQ. Each queue has its own configuration.
- **Consume** (`[]AMQPConsumeConfig`): Defines the consumers and how they should consume messages from RabbitMQ.

### `QueueConfig`

```go
type QueueConfig struct {
    Name       string
    Durable    bool
    AutoDelete bool
    Exclusive  bool
    NoWait     bool
    Args       amqp.Table
}
```

- **Name** (`string`): The name of the RabbitMQ queue.
- **Durable** (`bool`): If `true`, the queue will survive broker restarts.
- **AutoDelete** (`bool`): If `true`, the queue will be automatically deleted when the last consumer disconnects.
- **Exclusive** (`bool`): If `true`, the queue can only be used by the current connection and will be deleted when the connection is closed.
- **NoWait** (`bool`): If `true`, the server will not respond to the queue declaration. The client won’t wait for confirmation that the queue was created.
- **Args** (`amqp.Table`): Custom arguments to pass when creating the queue. Usually `nil`.

### `AMQPConsumeConfig`

```go
type AMQPConsumeConfig struct {
    Queue     string
    Consumer  string
    AutoAck   bool
    Exclusive bool
    NoLocal   bool
    NoWait    bool
    Args      amqp.Table
}
```

- **Queue** (`string`): The name of the queue to consume from.
- **Consumer** (`string`): The consumer tag to identify this consumer.
- **AutoAck** (`bool`): If `true`, messages will be automatically acknowledged after being delivered. Otherwise, manual acknowledgment is required.
- **Exclusive** (`bool`): If `true`, the queue can only be consumed by this consumer.
- **NoLocal** (`bool`): If `true`, messages published on this connection are not delivered to this consumer (rarely used).
- **NoWait** (`bool`): If `true`, the server will not send a response to the consumer setup request.
- **Args** (`amqp.Table`): Additional arguments for consumer setup.

### Example `.env` file

```
RABBITMQ_DSN=amqp://guest:guest@localhost:5672/
```

### Logging

`gossiper` logs every message received and any errors encountered during message unmarshalling.

### Contributing

Contributions are welcome! Feel free to submit issues or pull requests to improve the package or its documentation.

---

With `gossiper`, managing RabbitMQ consumers and environment variables in Go projects becomes more straightforward. Enjoy using it!

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author
![PCWT Dev Logo](https://avatars.githubusercontent.com/u/168465239?s=50)
### [PCWT Dev](https://github.com/pieceowater-dev)