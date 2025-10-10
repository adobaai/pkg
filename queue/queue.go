// Package queue provides ficilities for working with message queues.
package queue

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrStopped is returned when the queue is stopped.
	ErrStopped = errors.New("stopped")
	// ErrFull is returned when the queue is full.
	ErrFull = errors.New("full")
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

// Server is an interface for long-running.
type Server interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// PubSub is an interface for the Publish–Subscribe pattern.
type PubSub[K comparable, E any] interface {
	Server
	Pub(context.Context, E) error
	Sub(context.Context, K) (Subscription[E], error)
}

type Subscription[E any] interface {
	Close() error
	Ch() <-chan E
}

type Pub[e any] interface {
	Pub(ctx context.Context, e Event) error
}

type EventType string

// Event is a common event design.
type Event struct {
	ID        string
	Type      EventType
	EntityID  string
	Entity    any
	CreatedAt time.Time
}
