// Package internal provides core functionality and utilities for the Gossiper system.
package internal

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

// Env is an alias for the env.Env type.
// Handles environment configurations and is used to manage app-specific environment settings.
type Env = env.Env

// EnvVars is a pointer to env.EnvVars, holding application environment variables globally.
var EnvVars = &env.EnvVars

/* NETWORK */

// AMQP is an alias for the infra.AMQP type.
// Handles AMQP messaging operations, especially for RabbitMQ, providing key functions for producers and consumers.
type AMQP = infra.AMQP

/* CONFIGURATION */

// Config is an alias for the conf.Config type.
// Manages global application configuration, including RabbitMQ and database configurations.
type Config = conf.Config

// EnvConfig is an alias for the conf.EnvConfig type.
// Used to define and validate the required environment variables for the application.
type EnvConfig = conf.EnvConfig

// QueueConfig is an alias for the conf.QueueConfig type.
// Specifies configurations for RabbitMQ queues, such as durability, exclusivity, and other settings.
type QueueConfig = conf.QueueConfig

// AMQPConsumerConfig is an alias for the conf.AMQPConsumerConfig type.
// Contains settings specific to AMQP consumers like queue names and consumer tags.
type AMQPConsumerConfig = conf.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the conf.AMQPConsumeConfig type.
// Manages configurations for message consumption (e.g., auto-acknowledge, exclusivity, etc.).
type AMQPConsumeConfig = conf.AMQPConsumeConfig

// DBPGConfig is an alias for the PostgreSQL database configuration.
type DBPGConfig = conf.DBPGConfig

// DatabaseConfig represents the overall database configuration.
type DatabaseConfig = conf.DatabaseConfig

/* TOOLS */

// Tools is an alias for tools.Tools.
// A set of helper functions used throughout the system, such as logging and data validation.
type Tools = tools.Tools

/* BOOTSTRAP */

// Bootstrap is an alias for boot.Bootstrap.
// Handles the application's initialization and setup processes, especially at startup.
type Bootstrap = boot.Bootstrap

/* PAGINATION */

// ToPaginated converts a list of items and a total count into a paginated response.
// This function simplifies the creation of paginated responses for APIs.
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{pagination.ToPaginated(items, count)}
}

// PaginatedEntity wraps the pagination.PaginatedEntity structure, simplifying its use.
type PaginatedEntity[T any] struct {
	pagination.PaginatedEntity[T]
}

// Predefined constants for common pagination page lengths.
// These values make it easy to implement paginated APIs with standard limits.
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

/* FILTERING */

// NewDefaultFilter creates a new filter for a given data type.
// This function wraps the filter.NewDefaultFilter method, making it easier to apply filtering logic.
func NewDefaultFilter[T any]() t.DefaultFilter[T] {
	return filter.NewDefaultFilter[T]()
}

/* ERROR HANDLING */

// ServiceError is an alias for errors.ServiceError.
// This type is used to represent errors within the application.
type ServiceError = errors.ServiceError

// NewServiceError creates a new ServiceError instance.
// It accepts a message and an optional status code, facilitating structured error handling.
func NewServiceError(message string, statusCode ...int) *ServiceError {
	return errors.NewServiceError(message, statusCode...)
}
