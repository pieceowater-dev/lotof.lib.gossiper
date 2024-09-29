package gossiper

import (
	"github.com/fatih/color"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/internal/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	"log"
)

type Bootstrap struct {
}

// Setup initializes the Gossiper package with the provided configuration and sets up AMQP consumers.
// It logs the process, handles the setup of environment variables, and executes a startup function.
//
// Parameters:
//   - cfg: the configuration structure containing environment and AMQP settings.
//   - messageHandler: a callback function to handle incoming RabbitMQ messages.
//   - startupFunc: a function to execute after environment initialization.
func (b *Bootstrap) Setup(cfg config.Config, startupFunc func() any, messageHandler func([]byte) any) {
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	env := &environment.Env{}
	env.Init(cfg.Env.Required)

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)

	// Execute the provided startup function
	if startupFunc != nil {
		startupFunc()
	}

	net := &network.AMQP{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers(messageHandler)
}
