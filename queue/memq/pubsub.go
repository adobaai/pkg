package memq

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/adobaai/pkg/queue"
)

const (
	defaultPubCapacity = 1000
	defaultSubCapacity = 100
)

type pubSub[K comparable, E any] struct {
	idCounter int
	mu        sync.Mutex
	getKey    func(E) K
	subs      map[K]*sync.Map // map[id]Subscription
	events    chan E
	close     chan struct{}
	closed    atomic.Bool
}

type memSub[K comparable, E any] struct {
	ch     chan E
	cancel func()
}

func newMemSub[K comparable, E any](cancel func()) *memSub[K, E] {
	return &memSub[K, E]{
		ch:     make(chan E, defaultSubCapacity),
		cancel: cancel,
	}
}

func (ms *memSub[K, E]) Ch() <-chan E {
	return ms.ch
}

func (ms *memSub[K, E]) Cancel() error {
	ms.cancel()
	return nil
}

type newOption[K comparable, E any] struct {
	GetKey func(E) K
}

type Option[K comparable, E any] func(o *newOption[K, E])

func WithGetKey[K comparable, E any](getKey func(E) K) Option[K, E] {
	return func(o *newOption[K, E]) {
		o.GetKey = getKey
	}
}

// NewPubSub returns an in-memory implementation of the Publish–Subscribe pattern.
func NewPubSub[K comparable, E any](opts ...Option[K, E]) queue.PubSub[K, E] {
	var no newOption[K, E]
	for _, opt := range opts {
		opt(&no)
	}
	return &pubSub[K, E]{
		getKey: no.GetKey,
		subs:   make(map[K]*sync.Map),
		events: make(chan E, defaultPubCapacity),
		close:  make(chan struct{}),
	}
}

// Pub publishes event.
func (ps *pubSub[K, E]) Pub(ctx context.Context, e E) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ps.close:
		return queue.ErrStopped
	case ps.events <- e:
		return nil
	}
}

// Sub subscribes to the events.
func (ps *pubSub[K, E]) Sub(ctx context.Context, k K) (queue.Subscription[K, E], error) {
	select {
	case <-ps.close:
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
	cancel := func() {
		m.Delete(id)
	}
	sub := newMemSub[K, E](cancel)
	m.Store(id, sub)
	return sub, nil
}

func (ps *pubSub[K, E]) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-ps.events:
			key := ps.getKey(e)
			subMap, ok := ps.subs[key]
			if !ok {
				continue
			}
			subMap.Range(func(k, v any) bool {
				// TODO Receiver will block the publisher.
				v.(*memSub[K, E]).ch <- e
				return true
			})
		case <-ps.close:
			return queue.ErrStopped
		}
	}
}

func (ps *pubSub[K, E]) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if ps.closed.CompareAndSwap(false, true) {
		close(ps.close)
	}
	return nil
}
