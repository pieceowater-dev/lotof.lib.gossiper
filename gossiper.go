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

// Satisfies is an alias for the Tools.Satisfies method.
func Satisfies(data any, dest any) error {
	inst := Tools{}
	return inst.Satisfies(data, dest)
}

// LogAction is an alias for the Tools.LogAction method!
func LogAction(action string, data any) {
	inst := Tools{}
	inst.LogAction(action, data)
}

// NewServiceError is an alias for the Tools.NewServiceError method.
func NewServiceError(message string) *tools.ServiceError {
	return tools.NewServiceError(message)
}

func ToPaginated[T any](items []T, count int) tools.PaginatedEntity[T] {
	return tools.ToPaginated[T](items, count)
}

type DefaultFilter[T any] struct {
	tools.DefaultFilter[T]
}

func NewFilter[T any]() tools.DefaultFilter[T] {
	return tools.NewDefaultFilter[T]()
}

const (
	TEN          = tools.TEN
	FIFTEEN      = tools.FIFTEEN
	TWENTY       = tools.TWENTY
	TWENTY_FIVE  = tools.TWENTY_FIVE
	THIRTY       = tools.THIRTY
	THIRTY_FIVE  = tools.THIRTY_FIVE
	FORTY        = tools.FORTY
	FORTY_FIVE   = tools.FORTY_FIVE
	FIFTY        = tools.FIFTY
	FIFTY_FIVE   = tools.FIFTY_FIVE
	SIXTY        = tools.SIXTY
	SIXTY_FIVE   = tools.SIXTY_FIVE
	SEVENTY      = tools.SEVENTY
	SEVENTY_FIVE = tools.SEVENTY_FIVE
	EIGHTY       = tools.EIGHTY
	EIGHTY_FIVE  = tools.EIGHTY_FIVE
	NINETY       = tools.NINETY
	NINETY_FIVE  = tools.NINETY_FIVE
	ONE_HUNDRED  = tools.ONE_HUNDRED
)

// Setup initializes the Gossiper package with the provided configuration and sets up AMQP consumers.
// It logs the process, handles the setup of environment variables, and executes a startup function.
//
// Parameters:
//   - cfg: the configuration structure containing environment and AMQP settings.
//   - messageHandler: a callback function to handle incoming RabbitMQ messages.
//   - startupFunc: a function to execute after environment initialization.
func Setup(cfg config.Config, startupFunc func() any, messageHandler func([]byte) any) {
	_ = EnvVars

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
