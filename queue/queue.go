// Package queue provides ficilities for working with message queues.
package queue

import (
	"context"
	"errors"
	"time"
)

var (
	ErrStopped = errors.New("stopped")
)

// M refers the design of RabbitMQ message:
//
// - https://pkg.go.dev/github.com/rabbitmq/amqp091-go#Delivery
// - https://pkg.go.dev/github.com/segmentio/kafka-go#Message
// - https://pkg.go.dev/github.com/confluentinc/confluent-kafka-go/v2/kafka#Message

// M is a queue message.
type M struct {
	ID            string
	ContentType   string
	ContentLength int // In bytes
	CreatedAt     time.Time
	Metadata      Metadata
	Body          []byte
}

type Metadata map[string]string

// PubSub is an interface for the Publish–Subscribe pattern.
type PubSub[K comparable, E any] interface {
	Start(context.Context) error
	Stop(context.Context) error
	Pub(context.Context, E) error
	Sub(context.Context, K) (Subscription[K, E], error)
}

type Subscription[K comparable, E any] interface {
	Cancel() error
	Ch() <-chan E
}
