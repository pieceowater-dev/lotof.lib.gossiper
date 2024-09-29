package gossiper

import (
	bootstrap "github.com/pieceowater-dev/lotof.lib.gossiper/internal/bootstrap"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/internal/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
	t "github.com/pieceowater-dev/lotof.lib.gossiper/types"
)

// ENVIRONMENT

// Env is an alias for the environment.Env
type Env = environment.Env

// EnvVars is a pointer alias for the &environment.EnvVars
var EnvVars = &environment.EnvVars

// NETWORK

// AMQP is an alias for the network.AMQP
type AMQP = network.AMQP

// AMQMessage is an alias for the network.DefaultMessage
type AMQMessage = network.DefaultMessage

// CONFIG

// Config is an alias for the config.Config
type Config = config.Config

// EnvConfig is an alias for the config.EnvConfig
type EnvConfig = config.EnvConfig

// QueueConfig is an alias for the config.QueueConfig
type QueueConfig = config.QueueConfig

// AMQPConsumerConfig is an alias for the config.AMQPConsumerConfig
type AMQPConsumerConfig = config.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the config.AMQPConsumeConfig
type AMQPConsumeConfig = config.AMQPConsumeConfig

// TOOLS

// Tools is an alias for the tools.Tools
type Tools = tools.Tools

// Satisfies is an alias for the Tools.Satisfies method.
func Satisfies(data any, dest any) error {
	inst := Tools{}
	return inst.Satisfies(data, dest)
}

// LogAction is an alias for the Tools.LogAction method.
func LogAction(action string, data any) {
	inst := Tools{}
	inst.LogAction(action, data)
}

// NewServiceError is an alias for the Tools.NewServiceError method.
func NewServiceError(message string) *tools.ServiceError {
	return tools.NewServiceError(message)
}

// Enum with aliases for predefined pagination page length
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

// PaginatedEntity is a wrapper for tools.PaginatedEntity
type PaginatedEntity[T any] struct {
	tools.PaginatedEntity[T]
}

// NewFilter creates a new DefaultFilter instance.
func NewFilter[T any]() t.DefaultFilter[T] {
	return tools.NewDefaultFilter[T]()
}

// ToPaginated PaginatedEntity directly uses tools.PaginatedEntity
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{tools.ToPaginated[T](items, count)}
}

// BOOTSTRAP

type Bootstrap = bootstrap.Bootstrap

// Setup is an alias for the Bootstrap.Setup method.
func Setup(cfg config.Config, startupFunc func() any, messageHandler func([]byte) any) {
	b := Bootstrap{}
	b.Setup(cfg, startupFunc, messageHandler)
}
