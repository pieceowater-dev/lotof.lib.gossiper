package boot

import (
	"github.com/fatih/color"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra/db/pg"
	"log"
)

type Bootstrap struct {
	DB *pg.PGDB
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

	// Initialize environment variables
	envInst := &env.Env{}
	envInst.Init(cfg.Env.Required)

	// Initialize the database
	b.DB = pg.NewPGDB(cfg.Database.PG)
	b.DB.InitDB()

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)

	// Execute the provided startup function if it exists
	if startupFunc != nil {
		startupFunc()
	}

	// Setup AMQP Consumers
	net := &infra.AMQP{ConsumerConfig: cfg.AMQPConsumer}
	net.SetupAMQPConsumers(messageHandler)
}
