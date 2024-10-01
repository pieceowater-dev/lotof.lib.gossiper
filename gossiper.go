package gossiper

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal"
	t "github.com/pieceowater-dev/lotof.lib.gossiper/types"
)

/* ENVIRONMENT */

// Env is an alias for the internal.Env.
type Env = internal.Env

// EnvVars is a pointer alias for the internal.EnvVars.
var EnvVars = &internal.EnvVars

/* NETWORK */

// AMQP is an alias for the internal.AMQP.
type AMQP = internal.AMQP

// AMQMessage is an alias for the internal.DefaultMessage.
type AMQMessage = internal.DefaultMessage

/* CONFIG */

// Config is an alias for the internal.Config.
type Config = internal.Config

// EnvConfig is an alias for the internal.EnvConfig.
type EnvConfig = internal.EnvConfig

// QueueConfig is an alias for the internal.QueueConfig.
type QueueConfig = internal.QueueConfig

// AMQPConsumerConfig is an alias for the internal.AMQPConsumerConfig.
type AMQPConsumerConfig = internal.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the internal.AMQPConsumeConfig.
type AMQPConsumeConfig = internal.AMQPConsumeConfig

/* TOOLS */

// Tools is an alias for the tools.Tools.
type Tools = internal.Tools

// Satisfies checks if the data satisfies the destination structure.
func Satisfies(data any, dest any) error {
	inst := Tools{}
	return inst.Satisfies(data, dest)
}

// LogAction logs an action with the provided data.
func LogAction(action string, data any) {
	inst := Tools{}
	inst.LogAction(action, data)
}

// NewServiceError creates a new internal.ServiceError instance.
func NewServiceError(message string) *internal.ServiceError {
	return internal.NewServiceError(message)
}

// Enum with aliases for predefined pagination page length.
const (
	TEN          = internal.TEN
	FIFTEEN      = internal.FIFTEEN
	TWENTY       = internal.TWENTY
	TWENTY_FIVE  = internal.TWENTY_FIVE
	THIRTY       = internal.THIRTY
	THIRTY_FIVE  = internal.THIRTY_FIVE
	FORTY        = internal.FORTY
	FORTY_FIVE   = internal.FORTY_FIVE
	FIFTY        = internal.FIFTY
	FIFTY_FIVE   = internal.FIFTY_FIVE
	SIXTY        = internal.SIXTY
	SIXTY_FIVE   = internal.SIXTY_FIVE
	SEVENTY      = internal.SEVENTY
	SEVENTY_FIVE = internal.SEVENTY_FIVE
	EIGHTY       = internal.EIGHTY
	EIGHTY_FIVE  = internal.EIGHTY_FIVE
	NINETY       = internal.NINETY
	NINETY_FIVE  = internal.NINETY_FIVE
	ONE_HUNDRED  = internal.ONE_HUNDRED
)

// PaginatedEntity wraps internal.PaginatedEntity for convenience.
type PaginatedEntity[T any] struct {
	internal.PaginatedEntity[T]
}

// NewFilter creates a new types.DefaultFilter instance.
func NewFilter[T any]() t.DefaultFilter[T] {
	return internal.NewDefaultFilter[T]()
}

// ToPaginated converts items and count to a PaginatedEntity.
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{internal.ToPaginated[T](items, count)}
}

/* BOOTSTRAP */

// Bootstrap is an alias for the internal.Bootstrap.
type Bootstrap = internal.Bootstrap

// Setup initializes the bootstrap with the given configuration and handlers.
func Setup(cfg internal.Config, startupFunc func() any, messageHandler func([]byte) any) {
	b := Bootstrap{}
	b.Setup(cfg, startupFunc, messageHandler)
}
