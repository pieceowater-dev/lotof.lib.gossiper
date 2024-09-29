package gossiper

import (
	bootstrap "github.com/pieceowater-dev/lotof.lib.gossiper/internal/bootstrap"
	config "github.com/pieceowater-dev/lotof.lib.gossiper/internal/config"
	network "github.com/pieceowater-dev/lotof.lib.gossiper/internal/consume/mqp"
	environment "github.com/pieceowater-dev/lotof.lib.gossiper/internal/environment"
	tools "github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
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

// DefaultFilter is an alias for the Tools.DefaultFilter method.
type DefaultFilter[T any] struct {
	tools.DefaultFilter[T]
}

// PaginatedEntity is a wrapper for tools.PaginatedEntity
type PaginatedEntity[T any] struct {
	tools.PaginatedEntity[T]
}

// NewFilter creates a new DefaultFilter instance.
func NewFilter[T any]() tools.DefaultFilter[T] {
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
