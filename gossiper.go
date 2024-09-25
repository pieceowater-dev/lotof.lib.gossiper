package gossiper

import (
	"github.com/fatih/color"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/environment"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/tools"
	"log"
)

type Env = environment.Env
type Net = network.Net
type Config = config.Config
type EnvConfig = config.EnvConfig
type QueueConfig = config.QueueConfig
type AMQPConsumerConfig = config.AMQPConsumerConfig
type AMQPConsumeConfig = config.AMQPConsumeConfig
type Tools = tools.Tools

// Setup initializes the package with the provided configuration
func Setup(cfg config.Config) {
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	// Initialize environment variables
	env := &environment.Env{}
	env.Init(cfg.Env.Required)

	// Setup RabbitMQ configuration based on the provided config
	net := &network.Net{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers()

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)
}
