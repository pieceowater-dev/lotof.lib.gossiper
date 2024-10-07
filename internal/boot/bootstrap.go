package boot

import (
	"github.com/fatih/color"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra/db/ch"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra/db/pg"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools/panics"
	"log"
)

type Bootstrap struct {
	PGDB       *pg.PGDB
	ClickHouse *ch.ClickHouseDB
}

// Setup initializes the Gossiper package with the provided configuration and sets up AMQP consumers.
// It logs the process, handles the setup of environment variables, and executes a startup function.
//
// Parameters:
//   - cfg: the configuration structure containing environment and AMQP settings.
//   - messageHandler: a callback function to handle incoming RabbitMQ messages.
//   - startupFunc: a function to execute after environment initialization.
func (b *Bootstrap) Setup(cfg conf.Config, startupFunc func() any, messageHandler func([]byte) any) {
	defer panics.DontPanic()
	color.Set(color.FgGreen)
	log.SetFlags(log.LstdFlags)
	log.Println("Setting up Gossiper...")

	// Initialize environment variables
	envInst := &env.Env{}
	envInst.Init(cfg.Env.Required)

	// Initialize the database
	if cfg.Database.PG.EnvPostgresDBDSN != "" {
		b.PGDB = pg.NewPGDB(cfg.Database.PG)
		b.PGDB.InitDB()
	}

	if cfg.Database.ClickHouse.EnvClickHouseDBDSN != "" {
		b.ClickHouse = ch.NewClickHouseDB(cfg.Database.ClickHouse)
		b.ClickHouse.InitDB()
	}

	color.Set(color.FgCyan)
	log.Println("Setup complete.")
	color.Set(color.Reset)

	// Execute the provided startup function if it exists
	if startupFunc != nil {
		startupFunc()
	}

	// Setup AMQP Consumers
	if len(cfg.AMQPConsumer.Consume) != 0 {
		net := &infra.AMQP{ConsumerConfig: cfg.AMQPConsumer}
		net.SetupAMQPConsumers(messageHandler)
	}
}
