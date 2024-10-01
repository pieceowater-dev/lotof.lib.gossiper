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

// Env is an alias for the env.Env type, which manages environment configurations.
type Env = env.Env

// EnvVars is a pointer to the env.EnvVars structure that holds environment variables.
var EnvVars = &env.EnvVars

// AMQP is an alias for the infra.AMQP type, providing methods for AMQP messaging.
type AMQP = infra.AMQP

// Config is an alias for the conf.Config type, which holds application configuration settings.
type Config = conf.Config

// EnvConfig is an alias for the conf.EnvConfig type, containing environment-specific configurations.
type EnvConfig = conf.EnvConfig

// QueueConfig is an alias for the conf.QueueConfig type, representing queue-related configurations.
type QueueConfig = conf.QueueConfig

// AMQPConsumerConfig is an alias for the conf.AMQPConsumerConfig type, holding configurations for AMQP consumers.
type AMQPConsumerConfig = conf.AMQPConsumerConfig

// AMQPConsumeConfig is an alias for the conf.AMQPConsumeConfig type, providing configurations for AMQP consumption.
type AMQPConsumeConfig = conf.AMQPConsumeConfig

// Tools is an alias for the tools.Tools type, providing utility functions for various operations.
type Tools = tools.Tools

// Bootstrap is an alias for the boot.Bootstrap type, which handles application initialization processes.
type Bootstrap = boot.Bootstrap

// ToPaginated converts a slice of items and their count into a PaginatedEntity.
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	return PaginatedEntity[T]{pagination.ToPaginated(items, count)}
}

// PaginatedEntity is a wrapper around pagination.PaginatedEntity for easier usage in Gossiper.
type PaginatedEntity[T any] struct {
	pagination.PaginatedEntity[T]
}

// NewDefaultFilter creates a new types.DefaultFilter instance using the filter package.
func NewDefaultFilter[T any]() t.DefaultFilter[T] {
	return filter.NewDefaultFilter[T]()
}

// Predefined constants for pagination page lengths, allowing for easy configuration.
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

// ServiceError is an alias for the errors.ServiceError type, representing application-specific errors.
type ServiceError = errors.ServiceError

// NewServiceError creates a new instance of errors.ServiceError with the provided message.
func NewServiceError(message string, statusCode ...int) *ServiceError {
	return errors.NewServiceError(message, statusCode...)
}
