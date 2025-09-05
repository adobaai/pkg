package memq

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/adobaai/pkg/queue"
)

const (
	defaultPubCapacity = 1000
	defaultSubCapacity = 100
)

type pubSub[K comparable, E any] struct {
	subCap    uint
	idCounter int
	mu        sync.Mutex
	getKey    func(E) K
	subs      map[K]*sync.Map // map[id]Subscription
	events    chan E
	closeCh   chan struct{}
	closed    atomic.Bool
	logger    *slog.Logger
}

type memSub[K comparable, E any] struct {
	ch    chan E
	close func()
}

func newMemSub[K comparable, E any](capacity uint, cl func()) *memSub[K, E] {
	return &memSub[K, E]{
		ch:    make(chan E, capacity),
		close: cl,
	}
}

func (ms *memSub[K, E]) Ch() <-chan E {
	return ms.ch
}

func (ms *memSub[K, E]) Close() error {
	ms.close()
	return nil
}

type newOption struct {
	pubCapacity uint
	subCapacity uint
	logger      *slog.Logger
}

type Option func(o *newOption)

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option {
	return func(o *newOption) {
		o.logger = log.With("component", "memq")
	}
}

// WithPubCapacity sets the capacity of the publisher channel.
// Larger values allow more messages to be buffered before blocking publishers.
func WithPubCapacity(capacity uint) Option {
	return func(o *newOption) {
		o.pubCapacity = capacity
	}
}

// WithSubCapacity sets the capacity of each subscriber channel.
// Larger values allow more messages to be buffered per subscriber before dropping messages.
func WithSubCapacity(capacity uint) Option {
	return func(o *newOption) {
		o.subCapacity = capacity
	}
}

// NewPubSub returns an in-memory implementation of the Publish–Subscribe pattern.
//
// The getKey function extracts a routing key from messages
// to determine which subscribers receive them.
//
// Example:
//
//	pb := NewPubSub(func(msg *MyMessage) string { return msg.Topic })
//	go pb.Start(ctx)
//	pb.Pub(ctx, &MyMessage{Topic: "news", Content: "Hello"})
func NewPubSub[K comparable, E any](getKey func(E) K, opts ...Option) queue.PubSub[K, E] {
	no := newOption{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(&no)
	}

	if no.pubCapacity == 0 {
		no.pubCapacity = defaultPubCapacity
	}
	if no.subCapacity == 0 {
		no.subCapacity = defaultSubCapacity
	}

	return &pubSub[K, E]{
		subCap:  no.subCapacity,
		getKey:  getKey,
		subs:    make(map[K]*sync.Map),
		events:  make(chan E, no.pubCapacity),
		closeCh: make(chan struct{}),
		logger:  no.logger,
	}
}

// Pub publishes event.
func (ps *pubSub[K, E]) Pub(ctx context.Context, e E) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ps.closeCh:
		return queue.ErrStopped
	default:
	}
	// We should have two select to return `queue.ErrStopped` if the server is stopped.
	select {
	case ps.events <- e:
		return nil
	default:
		return queue.ErrFull
	}
}

// Sub subscribes to the events.
func (ps *pubSub[K, E]) Sub(ctx context.Context, k K) (queue.Subscription[E], error) {
	select {
	case <-ps.closeCh:
		return nil, queue.ErrStopped
	default:
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()

	m, ok := ps.subs[k]
	if !ok {
		m = &sync.Map{}
		ps.subs[k] = m
	}
	ps.idCounter++
	id := ps.idCounter
	cl := func() {
		m.Delete(id)
	}
	sub := newMemSub[K, E](ps.subCap, cl)
	m.Store(id, sub)
	return sub, nil
}

func (ps *pubSub[K, E]) Start(ctx context.Context) error {
	if ps.closed.Load() {
		return queue.ErrStopped
	}
	for {
		select {
		case <-ctx.Done():
			ps.close()
			return ctx.Err()
		case e := <-ps.events:
			key := ps.getKey(e)
			subMap, ok := ps.subs[key]
			if !ok {
				continue
			}
			subMap.Range(func(k, v any) bool {
				// Use non-blocking send to prevent publisher from blocking
				select {
				case v.(*memSub[K, E]).ch <- e:
				default:
					// OPTI: no default logger
					ps.logger.WarnContext(ctx, "message dropped", "key", key, "event", e)
				}
				return true
			})
		case <-ps.closeCh:
			return nil
		}
	}
}

func (ps *pubSub[K, E]) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		ps.close()
		return nil
	}
}

func (ps *pubSub[K, E]) close() {
	if ps.closed.CompareAndSwap(false, true) {
		close(ps.closeCh)
	}
}
