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

	"github.com/adobaai/pkg/collections"
)

// TODO move out

type M[T any] struct {
	ID            string
	ContentType   string // Default to [MIMEApplicationJSON]
	ContentLength int    // In bytes
	CreatedAt     time.Time
	Metadata      Metadata
	Body          T
}

func MessageV2ToRedisValues[T any](m *M[T]) (res []any, err error) {
	var body []byte
	var meta []byte
	switch m.ContentType {
	default:
		return nil, fmt.Errorf("%w: %s", UnsupportedContentTyep, m.ContentType)
	case "", MIMEApplicationJSON:
		meta, err = json.Marshal(m.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		body, err = json.Marshal(m.Body)
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

func ToMessageV2[T any](m M2) (res *M[T], err error) {
	createdAt, err := time.Parse(time.RFC3339Nano, m.GetValue("ca"))
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	cl, err := strconv.Atoi(m.GetValue("cl"))
	if err != nil {
		return nil, fmt.Errorf("parse contentLength: %w", err)
	}

	var body T
	meta := Metadata{}
	ct := m.GetValue("ct")
	metaStr := m.GetValue("mt")
	bodyStr := m.GetValue("bd")
	switch ct {
	default:
		return nil, fmt.Errorf("%w: %s", UnsupportedContentTyep, ct)
	case "", MIMEApplicationJSON:
		if err = json.Unmarshal([]byte(metaStr), &meta); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		if err = json.Unmarshal([]byte(bodyStr), &body); err != nil {
			return nil, fmt.Errorf("unmarshal body: %w", err)
		}
	}
	return &M[T]{
		ID:            string(m.ID),
		ContentType:   ct,
		ContentLength: cl,
		CreatedAt:     createdAt,
		Metadata:      meta,
		Body:          body,
	}, nil
}

var (
	UnsupportedContentTyep = errors.New("unsupported content type")
)

var (
	MIMEApplicationJSON = "application/json"
)

type MessageID string

type M2 struct {
	ID     string
	Values map[string]any
}

func FromRedisMessage(xm redis.XMessage) M2 {
	return M2{
		ID:     xm.ID,
		Values: xm.Values,
	}
}

func (m M2) GetValue(key string) string {
	v, ok := m.Values[key]
	if !ok {
		return ""
	}
	return v.(string)
}

type Metadata map[string]string

// Add adds a new message to the given stream.
func Add[T any](ctx context.Context, rdb *redis.Client, stream string, m *M[T]) error {
	values, err := MessageV2ToRedisValues(m)
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

type HandleResV2 struct {
	// IDs will be acknowledged
	OKIDs []MessageID
	// The metadata that will be set when put into the dead letter queue
	Metadata map[MessageID]Metadata
}

func (res *HandleResV2) GetMetadata(id MessageID) Metadata {
	if res.Metadata == nil {
		return nil
	} else {
		return res.Metadata[id]
	}
}

func (res *HandleResV2) SetMetadata(id MessageID, m Metadata) {
	if res.Metadata == nil {
		res.Metadata = map[MessageID]Metadata{id: m}
	} else {
		res.Metadata[id] = m
	}
}

type Context interface {
	context.Context
	WithContext(context.Context) Context
	Route() Route
	Msg() M2
}

// BatchHandlerV2 is the version 2 batch handler.
// The handler will only retry when err != nil.
type BatchHandlerV2 func(ctx context.Context, ms []M2) (res *HandleResV2, err error)

type myContext struct {
	context.Context
	route *Route
	msg   M2
}

func (mc *myContext) WithContext(ctx context.Context) Context {
	c2 := *mc
	c2.Context = ctx
	return &c2
}

func (mc *myContext) Msg() M2 {
	return mc.msg
}

func (mc *myContext) Route() Route {
	return *mc.route
}

func NewContext(ctx context.Context, r *Route, m M2) Context {
	return &myContext{
		Context: ctx,
		route:   r,
		msg:     m,
	}
}

type Route struct {
	Stream    string
	Group     string
	ID        string
	Handler   Handler
	NoPending bool
}

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
	for _, r := range c.routes {
		go c.loopRoute(ctx, r)
	}
	<-ctx.Done()
	return nil
}

func (c *Consumer) Stop(ctx context.Context) error {
	c.cancel()
	select {
	case <-c.ctx.Done():
		// TODO check it context.Canceled
		return c.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) MustAddRoute(r *Route) {
	if r.Handler == nil {
		panic("redisq: no handler provide")
	}

	if r.ID == "" {
		if r.NoPending {
			r.ID = ">"
		} else {
			r.ID = "0"
		}
	}
	c.routes = append(c.routes, r)
}

func MustAddHandler[T any](
	c *Consumer,
	r *Route,
	h func(ctx Context, m *M[T]) error,
) {
	r.Handler = func(ctx Context) error {
		mv2, err := ToMessageV2[T](ctx.Msg())
		if err != nil {
			return fmt.Errorf("to msgv2: %w", err)
		}
		return h(ctx, mv2)
	}
	c.routes = append(c.routes, r)
}

func (c *Consumer) loopRoute(ctx context.Context, r *Route) {
	l := c.logger.With("stream", r.Stream, "group", r.Group)

	// TODO Distinguish between framework errors and business errors
	do := func() {
		state, err := c.handleRoute(ctx, r)
		if err == nil {
			return
		}
		if errors.Is(err, redis.Nil) {
			l.DebugContext(ctx, "redis nil", "state", "sleep")
			time.Sleep(time.Minute)
		} else {
			l.ErrorContext(ctx, err.Error(), "state", state)
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

func (c *Consumer) handleRoute(ctx context.Context, r *Route) (state string, err error) {
	ms, state, err := c.readCheck(ctx, r)
	if err != nil {
		return
	}

	for _, m := range ms {
		ctx := NewContext(ctx, r, m)
		h := Chain(c.mws...)(r.Handler)
		if err := h(ctx); err != nil {
			return "", err
		}

		state = "ack"
		err = c.client.XAck(ctx, r.Stream, r.Group, m.ID).Err()
		if err != nil {
			return
		}
	}
	return
}

func (c *Consumer) readCheck(ctx context.Context, r *Route,
) (ms []M2, state string, err error) {
	// By design: Deleted entries still show up in xpending.
	// See https://github.com/redis/redis/issues/6199
	for {
		ms, state, err = c.read(ctx, r)
		if err != nil {
			return
		}

		msGroup := lo.GroupBy(ms, func(it M2) bool {
			return it.Values == nil
		})
		if deletedXMs := msGroup[true]; len(deletedXMs) > 0 {
			ids := lo.Map(deletedXMs, func(x M2, n int) string { return x.ID })
			state = "ackDeleted"
			cmd := c.client.XAck(ctx, r.Stream, r.Group, ids...)
			if err = cmd.Err(); err != nil {
				return
			}
		}
		ms = msGroup[false]
		if len(ms) > 0 {
			break
		}
	}
	return
}

func (c *Consumer) read(ctx context.Context, r *Route) (ms []M2, state string, err error) {
	state = "init"
	var (
		xss      []redis.XStream
		count    int64 = 1
		consumer       = "c1"
	)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Use any other ID (besides '>') to return all entries that are pending.
	// See https://redis.io/commands/xreadgroup/.
	xss, err = c.client.XReadGroup(readCtx, &redis.XReadGroupArgs{
		Group:    r.Group,
		Consumer: consumer,
		Count:    count,
		Streams:  []string{r.Stream, r.ID},
	}).Result()
	if err != nil {
		return
	}

	// If no pending entries, xss is "[{stream []}]".
	ms = collections.Map(xss[0].Messages, FromRedisMessage)
	// TODO test it
	return
}

// TODO Queue size
// TODO Middleware
