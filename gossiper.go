package gossiper

import (
	"github.com/fatih/color"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/internal/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
	"log"
)

// EnvVars is a global reference to the environment variables map initialized by the Env handler.
var EnvVars = &environment.EnvVars

// Type aliases for simplified usage throughout the package

type Env = environment.Env
type AMQP = network.AMQP
type AMQMessage = network.DefaultMessage
type Config = config.Config
type EnvConfig = config.EnvConfig
type QueueConfig = config.QueueConfig
type AMQPConsumerConfig = config.AMQPConsumerConfig
type AMQPConsumeConfig = config.AMQPConsumeConfig
type Tools = tools.Tools

// Setup initializes the Gossiper package with the provided configuration and sets up AMQP consumers.
// It logs the process and handles the setup of environment variables and RabbitMQ consumers.
//
// Parameters:
//   - cfg: the configuration structure containing environment and AMQP settings.
//   - messageHandler: a callback function to handle incoming RabbitMQ messages.
func Setup(cfg config.Config, messageHandler func([]byte) any) {
	// Reference EnvVars to make sure it's initialized.
	_ = EnvVars

	// Log the setup process with green text for visual indication.
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags) // Set standard logging flags.
	log.Println("Setting up Gossiper...")

	// Initialize environment variables based on the provided configuration.
	env := &environment.Env{}
	env.Init(cfg.Env.Required)

	// Indicate the setup completion with cyan text.
	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset) // Reset text color back to default.

	// Setup RabbitMQ consumers using the AMQP configuration and provided message handler.
	net := &network.AMQP{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers(messageHandler)
}
