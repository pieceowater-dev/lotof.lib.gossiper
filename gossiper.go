package gossiper

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal"
	t "github.com/pieceowater-dev/lotof.lib.gossiper/types"
)

/* ENVIRONMENT */

// Env is an alias for the internal.Env.
// Provides access to environment-related utilities.
type Env = internal.Env

// EnvVars is a pointer alias for the internal.EnvVars.
// Used to manage environment variables globally.
var EnvVars = &internal.EnvVars

/* NETWORK */

// AMQP is an alias for the internal.AMQP.
// Provides RabbitMQ or AMQP related functions and configurations.
type AMQP = internal.AMQP

// AMQMessage is an alias for the internal.DefaultMessage.
// Represents the structure of messages that are exchanged over AMQP.
type AMQMessage = t.DefaultMessage

/* CONFIG */

// Config is an alias for the internal.Config.
// Represents the general application configuration structure.
type Config = internal.Config

// EnvConfig is an alias for the internal.EnvConfig.
// Contains settings related to environment variables.
type EnvConfig = internal.EnvConfig

// QueueConfig is an alias for the internal.QueueConfig.
// Defines configuration for RabbitMQ queues.
type QueueConfig = internal.QueueConfig

// AMQPConsumerConfig is an alias for the internal.AMQPConsumerConfig.
// Holds the RabbitMQ consumer-specific configuration settings.
type AMQPConsumerConfig = internal.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the internal.AMQPConsumeConfig.
// Describes how messages are consumed from RabbitMQ queues.
type AMQPConsumeConfig = internal.AMQPConsumeConfig

// DBPGConfig is an alias for the internal.DBPGConfig.
// Defines PostgreSQL database configuration options.
type DBPGConfig = internal.DBPGConfig

// DBClickHouseConfig is an alias for the internal.DBClickHouseConfig.
// Defines ClickHouse database configuration options.
type DBClickHouseConfig = internal.DBClickHouseConfig

// DatabaseConfig is an alias for the internal.DatabaseConfig.
// Groups database configuration settings (e.g., PostgreSQL settings).
type DatabaseConfig = internal.DatabaseConfig

/* TOOLS */

// Tools is an alias for the tools.Tools from the internal package.
// Provides various utility functions for the application.
type Tools = internal.Tools

// Satisfies checks if the provided data conforms to the destination structure.
// Useful for verifying if a structure meets the required interface or type.
func Satisfies(data any, dest any) error {
	inst := Tools{}
	return inst.Satisfies(data, dest)
}

// LogAction logs an action with the provided data.
// A simple wrapper for logging actions within the application.
func LogAction(action string, data any) {
	inst := Tools{}
	inst.LogAction(action, data)
}

// NewServiceError creates a new instance of internal.ServiceError.
// Used to generate a service-related error with an optional status code.
func NewServiceError(message string, statusCode ...int) *internal.ServiceError {
	return internal.NewServiceError(message, statusCode...)
}

// Enum with aliases for predefined pagination page length.
// These constants define common pagination limits (e.g., 10, 20, 50 items per page).
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
// A generic structure for paginated results, supporting any type `T`.
type PaginatedEntity[T any] struct {
	internal.PaginatedEntity[T]
}

// NewFilter creates a new types.DefaultFilter instance.
// This function initializes a default filter for any data type `T`.
func NewFilter[T any]() t.DefaultFilter[T] {
	return internal.NewDefaultFilter[T]()
}

// ToPaginated converts items and count to a PaginatedEntity.
// Wraps a list of items and a count into a paginated entity for easier response formatting.
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{internal.ToPaginated[T](items, count)}
}

// DontPanic is a wrapper for internal.DontPanic.
// It allows the application to recover from panics in the calling context.
func DontPanic() {
	internal.DontPanic()
}

// Safely executes a function with panic recovery.
// It returns any errors that occur during execution, including panics.
//
// Parameters:
//
//	fn - A function to be executed safely.
//
// Returns:
//
//	An error if a panic occurred; otherwise, nil.
func Safely(fn func()) (err error) {
	return internal.Safely(fn)
}

/* BOOTSTRAP */

// Bootstrap is an alias for the internal.Bootstrap.
// This is used to set up the core of the application.
type Bootstrap = internal.Bootstrap

// Setup initializes the bootstrap with the given configuration and handlers.
// This is where the application is configured, including startup logic and message handling.
func Setup(cfg internal.Config, startupFunc func() any, messageHandler func([]byte) any) {
	b := Bootstrap{}
	b.Setup(cfg, startupFunc, messageHandler)
}
