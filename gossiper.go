package gossiper

import (
	"github.com/fatih/color"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/environment"
	"log"
)

type Cfg = config.Config
type EnvCfg = config.EnvConfig
type QueueCfg = config.QueueConfig
type AMQPConsumerCfg = config.AMQPConsumerConfig
type AMQPConsumeCfg = config.AMQPConsumeConfig

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
