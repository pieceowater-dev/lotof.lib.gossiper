package infra

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/infra/amqp"
)

type AMQP = amqp.AMQP

func NewAMQPClient(queueName string, dsn string) (*amqp.Client, error) {
	return amqp.New(queueName, dsn)
}
