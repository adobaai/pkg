// Package redisq provides a Redis Stream based message queue.
package redisq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"

	"github.com/adobaai/pkg"
	"github.com/adobaai/pkg/collections"
	"github.com/adobaai/pkg/queue"
)

const (
	MIMEJSON = "application/json"
)

var (
	MaxLen int64 = 10000 // See the README about details
)

// RM is the raw Redis message.
type RM struct {
	ID     string
	Values map[string]any
}

func fromRedisMsg(xm redis.XMessage) RM {
	return RM{
		ID:     xm.ID,
		Values: xm.Values,
	}
}

func (m RM) GetStr(key string) string {
	v, ok := m.Values[key]
	if !ok {
		return ""
	}
	return v.(string)
}

type M[T any] struct {
	queue.M
	T T
}

// NewM creates a new default message.
func NewM[T any](t T) *M[T] {
	return &M[T]{
		M: queue.M{
			ContentType: MIMEJSON,
			CreatedAt:   time.Now(),
		},
		T: t,
	}
}

func (m *M[T]) toRedisValues() (res []any, err error) {
	var body []byte
	var meta []byte
	switch m.ContentType {
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrUnsupported, m.ContentType)
	case "", MIMEJSON:
		meta, err = json.Marshal(m.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		body, err = json.Marshal(m.T)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
	}
	return []any{
		"ct", m.ContentType,
		"cl", len(body),
		"ca", m.CreatedAt.Format(time.RFC3339Nano),
		"mt", meta,
		"bd", body,
	}, nil
}

// GPT: Since M[T] contains fields like Body, Metadata,
// and potentially large data (e.g., JSON-encoded body),
// returning a pointer (*M[T]) is the better choice for efficiency and consistency.

func toM2[T any](m RM) (res *M[T], err error) {
	createdAt, err := time.Parse(time.RFC3339Nano, m.GetStr("ca"))
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	cl, err := strconv.Atoi(m.GetStr("cl"))
	if err != nil {
		return nil, fmt.Errorf("parse contentLength: %w", err)
	}

	var t T
	meta := queue.Metadata{}
	ct := m.GetStr("ct")
	metaStr := m.GetStr("mt")
	bodyStr := m.GetStr("bd")
	switch ct {
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrUnsupported, ct)
	case "", MIMEJSON:
		if err = json.Unmarshal([]byte(metaStr), &meta); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		if err = json.Unmarshal([]byte(bodyStr), &t); err != nil {
			return nil, fmt.Errorf("unmarshal body: %w", err)
		}
	}
	return &M[T]{
		M: queue.M{
			ID:            m.ID,
			ContentType:   ct,
			ContentLength: cl,
			CreatedAt:     createdAt,
			Metadata:      meta,
			Body:          []byte(bodyStr),
		},
		T: t,
	}, nil
}

// Publish publishes a new message to the given stream.
func Publish[T any](ctx context.Context, rdb *redis.Client, stream string, m *M[T]) error {
	values, err := m.toRedisValues()
	if err != nil {
		return fmt.Errorf("to redis values: %w", err)
	}

	if err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

type Context interface {
	context.Context
	WithContext(context.Context) Context
	Route() Route
	// IsBatch returns true if the route is batch.
	IsBatch() bool
	// Msg returns the first message in the current context.
	Msg() RM
	// Msgs returns all messages in the current context.
	Msgs() []RM
	// Ack acknowledge the messages.
	// If no IDs are provided, all messages will be acknowledged when error is nil.
	Ack(ids ...string)

	getAckIDs() []string
}

type myContext struct {
	context.Context
	route  *Route
	msgs   []RM
	ackIDs *pkg.Slice[string]
}

func (mc *myContext) WithContext(ctx context.Context) Context {
	c2 := *mc
	c2.Context = ctx
	return &c2
}

func (mc *myContext) IsBatch() bool {
	return mc.route.BatchSize > 1
}

func (mc *myContext) Msg() RM {
	return mc.msgs[0]
}

func (mc *myContext) Msgs() []RM {
	return mc.msgs
}

func (mc *myContext) Route() Route {
	return *mc.route
}

func (mc *myContext) Ack(ids ...string) {
	mc.ackIDs.Append(ids...)
}

func (mc *myContext) getAckIDs() []string {
	return mc.ackIDs.Get()
}

func newContext(ctx context.Context, r *Route, ms ...RM) Context {
	return &myContext{
		Context: ctx,
		route:   r,
		msgs:    ms,
		ackIDs:  &pkg.Slice[string]{},
	}
}

type Route struct {
	Stream    string
	Group     string
	PendingID string  // The start ID for pending messages, default is "0"
	Handler   Handler // Handler is the message handler
	NoPending bool    // NoPending ignores the pending messages
	BatchSize int64   // BatchSize specifies the number of messages fetched per batch
	MaxLen    int64   // MaxLen specifies the max length of current stream
}

// SpanName is the name of the span for tracing.
func (r *Route) SpanName() string {
	return fmt.Sprintf("/redisq/%s/%s", r.Stream, r.Group)
}

// Consumer is Redis Stream based message queue.
type Consumer struct {
	client *redis.Client
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc

	mws    []Middleware
	routes []*Route
}

type Option func(*Consumer)

func WithMiddlewares(mws ...Middleware) Option {
	return func(c *Consumer) {
		c.mws = append(c.mws, mws...)
	}
}

func NewConsumer(c *redis.Client, l *slog.Logger, opts ...Option) (res *Consumer) {
	res = &Consumer{
		client: c,
		logger: l.With("pkg", "redisq"),
	}
	for _, opt := range opts {
		opt(res)
	}
	return res
}

func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go c.trim()
	for _, r := range c.routes {
		go c.loopRoute(ctx, r)
	}
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func (c *Consumer) Stop(ctx context.Context) error {
	c.cancel()
	select {
	case <-c.ctx.Done():
		err := c.ctx.Err()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) MustAddRoute(r *Route) {
	if r.Handler == nil {
		panic("redisq: no handler provide")
	}

	if r.PendingID == "" {
		r.PendingID = "0"
	}
	if r.BatchSize == 0 {
		r.BatchSize = 1
	}
	if r.MaxLen == 0 {
		r.MaxLen = MaxLen
	}
	c.routes = append(c.routes, r)
}

func MustAddHandler[T any](
	c *Consumer,
	r *Route,
	h func(ctx Context, m *M[T]) error,
) {
	r.Handler = func(ctx Context) error {
		mv2, err := toM2[T](ctx.Msg())
		if err != nil {
			return fmt.Errorf("to msgv2: %w", err)
		}
		return h(ctx, mv2)
	}
	c.MustAddRoute(r)
}

func MustAddBatchHandler[T any](
	c *Consumer,
	r *Route,
	h func(ctx Context, ms []*M[T]) error,
) {
	r.Handler = func(ctx Context) error {
		var ms []*M[T]
		for i := range ctx.Msgs() {
			v, err := toM2[T](ctx.Msgs()[i])
			if err != nil {
				return err
			}
			ms = append(ms, v)
		}

		return h(ctx, ms)
	}
	c.MustAddRoute(r)
}

func (c *Consumer) loopRoute(ctx context.Context, r *Route) {
	l := c.logger.With("stream", r.Stream, "group", r.Group)

	// OPTI: Distinguish between framework errors and business errors
	do := func() {
		err := c.handleRoute(ctx, r)
		if err == nil {
			return
		}
		if errors.Is(err, redis.Nil) {
			l.DebugContext(ctx, "no message", "func", "loopRoute")
			time.Sleep(time.Minute)
		} else {
			l.ErrorContext(ctx, err.Error(), "func", "loopRoute")
			time.Sleep(3 * time.Second)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			do()
		}
	}
}

func (c *Consumer) handleRoute(ctx context.Context, r *Route) (err error) {
	ms, err := c.readCheck(ctx, r)
	if err != nil {
		return
	}

	myCtx := newContext(ctx, r, ms...)
	h := Chain(c.mws...)(r.Handler)
	err = h(myCtx)
	if ids := myCtx.getAckIDs(); len(ids) != 0 {
		err = errors.Join(
			err,
			c.client.XAck(ctx, r.Stream, r.Group, ids...).Err(),
		)
	} else if err == nil {
		ids = collections.Map(ms, getID)
		err = c.client.XAck(ctx, r.Stream, r.Group, ids...).Err()
	}
	return
}

func getID(m RM) string {
	return m.ID
}

// readCheck reads the messages and checks for deleted entries.
func (c *Consumer) readCheck(ctx context.Context, r *Route) (ms []RM, err error) {
	// By design: Deleted entries still show up in xpending.
	// See https://github.com/redis/redis/issues/6199
	for {
		ms, err = c.read(ctx, r)
		if err != nil {
			return
		}

		msGroup := lo.GroupBy(ms, func(it RM) bool {
			return it.Values == nil
		})
		if deletedXMs := msGroup[true]; len(deletedXMs) > 0 {
			ids := lo.Map(deletedXMs, func(x RM, n int) string { return x.ID })
			cmd := c.client.XAck(ctx, r.Stream, r.Group, ids...)
			if err = cmd.Err(); err != nil {
				return nil, fmt.Errorf("ack deleted: %w", err)
			}
		}
		ms = msGroup[false]
		if len(ms) > 0 {
			break
		}
	}
	return
}

func (c *Consumer) read(ctx context.Context, r *Route) (ms []RM, err error) {
	var (
		xss      []redis.XStream
		xms      []redis.XMessage
		consumer = "c1"
	)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if !r.NoPending {
		// Use any other ID (besides '>') to return all entries that are pending.
		// See https://redis.io/commands/xreadgroup/.
		xss, err = c.client.XReadGroup(readCtx, &redis.XReadGroupArgs{
			Group:    r.Group,
			Consumer: consumer,
			Streams:  []string{r.Stream, r.PendingID},
			Count:    r.BatchSize,
		}).Result()
		if err != nil {
			return nil, fmt.Errorf("read pending: %w", err)
		}
		// If no pending entries, xss is "[{stream []}]".
		// See TestXReadGroup for details.
		xms = xss[0].Messages
		r.NoPending = len(xms) < int(r.BatchSize)
	}
	if len(xms) != 0 {
		// The last item has the biggest id.
		r.PendingID = xms[len(xms)-1].ID
	} else {
		// If no data, err is "redis.Nil".
		xss, err = c.client.XReadGroup(readCtx, &redis.XReadGroupArgs{
			Group:    r.Group,
			Consumer: consumer,
			Streams:  []string{r.Stream, ">"},
			Count:    r.BatchSize,
			Block:    -1,
		}).Result()
		if err != nil {
			return nil, fmt.Errorf("read new: %w", err)
		}
		xms = xss[0].Messages
	}

	ms = collections.Map(xms, fromRedisMsg)
	return
}

// trim trims the streams to the max length.
func (c *Consumer) trim() {
	groups := lo.GroupBy(c.routes, func(it *Route) string { return it.Stream })
	trims := lo.MapEntries(groups, func(stream string, routes []*Route) (string, int64) {
		lens := collections.Map(routes, func(it *Route) int64 { return it.MaxLen })
		return stream, lo.Max(lens)
	})

	var (
		ctx      = c.ctx
		interval = 3 * time.Minute
		errCount = 0
		l        = c.logger.With("task", "trim")
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			for stream, maxLen := range trims {
				l := l.With("stream", stream)
				n, err := c.client.XTrimMaxLenApprox(ctx, stream, maxLen, 0).Result()
				if err == nil {
					errCount = 0
					l.DebugContext(ctx, "xtrim done", "count", n)
				} else {
					if errors.Is(err, context.Canceled) {
						return
					}
					errCount++
					l.ErrorContext(ctx, "trim error", "errCount", errCount, "err", err)
				}
			}
			time.Sleep(interval)
		}
	}

}
