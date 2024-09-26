package gossiper

import (
	"github.com/fatih/color"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/internal/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
	"log"
)

// EnvVars is a reference to environment variables configuration.
var EnvVars = &environment.EnvVars

type Env = environment.Env
type Net = network.Net
type AMQMessage = network.DefaultMessage
type Config = config.Config
type EnvConfig = config.EnvConfig
type QueueConfig = config.QueueConfig
type AMQPConsumerConfig = config.AMQPConsumerConfig
type AMQPConsumeConfig = config.AMQPConsumeConfig
type Tools = tools.Tools

// Setup initializes the package with the provided configuration
func Setup(cfg config.Config, messageHandler func([]byte) interface{}) {
	_ = EnvVars
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	// Initialize environment variables
	env := &environment.Env{}
	env.Init(cfg.Env.Required)

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)

	// Setup RabbitMQ consumers
	net := &network.Net{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers(messageHandler)
}
