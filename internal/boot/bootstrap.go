package boot

import (
	"github.com/fatih/color"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra"
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
func (b *Bootstrap) Setup(cfg conf.Config, startupFunc func() any, messageHandler func([]byte) any) {
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	envInst := &env.Env{}
	envInst.Init(cfg.Env.Required)

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)

	// Execute the provided startup function
	if startupFunc != nil {
		startupFunc()
	}

	net := &infra.AMQP{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers(messageHandler)
}
