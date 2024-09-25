package gossiper

import (
	"github.com/fatih/color"
	"github.com/pieceowater-dev/lotof.lib.gossiper/config"
	"github.com/pieceowater-dev/lotof.lib.gossiper/consume/mqp"
	"github.com/pieceowater-dev/lotof.lib.gossiper/environment"
	"log"
)

type Cfg = config.Config
type EnvCfg = config.EnvConfig
type QueueCfg = config.QueueConfig
type ConsumerCfg = config.ConsumerConfig
type ConsumeCfg = config.ConsumeConfig

// Setup initializes the package with the provided configuration
func Setup(config config.Config) {
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	// Initialize environment variables
	environment.Init(config.Env.Required)

	// Setup RabbitMQ configuration based on the provided config
	mqp.SetupConsumers(config.AMQPConsumer)

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)
}
