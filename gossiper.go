package gossiper

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/boot"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools/formats/errors"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools/formats/filter"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/tools/formats/pagination"
	t "github.com/pieceowater-dev/lotof.lib.gossiper/types"
)

/* ENVIRONMENT */

// Env is an alias for the environment.Env
type Env = env.Env

// EnvVars is a pointer alias for the &environment.EnvVars
var EnvVars = &env.EnvVars

/* NETWORK */

// AMQP is an alias for the network.AMQP
type AMQP = infra.AMQP

// AMQMessage is an alias for the infra.DefaultMessage
type AMQMessage = infra.DefaultMessage

/* CONFIG */

// Config is an alias for the config.Config
type Config = conf.Config

// EnvConfig is an alias for the config.EnvConfig
type EnvConfig = conf.EnvConfig

// QueueConfig is an alias for the config.QueueConfig
type QueueConfig = conf.QueueConfig

// AMQPConsumerConfig is an alias for the config.AMQPConsumerConfig
type AMQPConsumerConfig = conf.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the config.AMQPConsumeConfig
type AMQPConsumeConfig = conf.AMQPConsumeConfig

/* TOOLS */

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
func NewServiceError(message string) *errors.ServiceError {
	return errors.NewServiceError(message)
}

// Enum with aliases for predefined pagination page length
const (
	TEN          = filter.TEN
	FIFTEEN      = filter.FIFTEEN
	TWENTY       = filter.TWENTY
	TWENTY_FIVE  = filter.TWENTY_FIVE
	THIRTY       = filter.THIRTY
	THIRTY_FIVE  = filter.THIRTY_FIVE
	FORTY        = filter.FORTY
	FORTY_FIVE   = filter.FORTY_FIVE
	FIFTY        = filter.FIFTY
	FIFTY_FIVE   = filter.FIFTY_FIVE
	SIXTY        = filter.SIXTY
	SIXTY_FIVE   = filter.SIXTY_FIVE
	SEVENTY      = filter.SEVENTY
	SEVENTY_FIVE = filter.SEVENTY_FIVE
	EIGHTY       = filter.EIGHTY
	EIGHTY_FIVE  = filter.EIGHTY_FIVE
	NINETY       = filter.NINETY
	NINETY_FIVE  = filter.NINETY_FIVE
	ONE_HUNDRED  = filter.ONE_HUNDRED
)

// PaginatedEntity is a wrapper for tools.PaginatedEntity
type PaginatedEntity[T any] struct {
	pagination.PaginatedEntity[T]
}

// NewFilter creates a new DefaultFilter instance.
func NewFilter[T any]() t.DefaultFilter[T] {
	return filter.NewDefaultFilter[T]()
}

// ToPaginated PaginatedEntity directly uses tools.PaginatedEntity
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{pagination.ToPaginated[T](items, count)}
}

/* BOOTSTRAP */

type Bootstrap = boot.Bootstrap

// Setup is an alias for the Bootstrap.Setup method.
func Setup(cfg conf.Config, startupFunc func() any, messageHandler func([]byte) any) {
	b := Bootstrap{}
	b.Setup(cfg, startupFunc, messageHandler)
}
