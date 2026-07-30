// Package redisq provides a Redis Stream based message queue.
package redisq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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

	pendingStartID string
	readNewNext    bool
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
	r.pendingStartID = r.PendingID
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
		xms      []redis.XMessage
		consumer = "c1"
	)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// After a pending batch, give new messages one non-blocking read before
	// continuing the pending scan. This keeps a poison message from starving
	// the rest of the stream.
	if r.readNewNext {
		r.readNewNext = false
		xms, err = c.readGroup(readCtx, r, consumer, ">")
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("read new: %w", err)
		}
		if len(xms) != 0 {
			return collections.Map(xms, fromRedisMsg), nil
		}
	}

	if !r.NoPending {
		// Any ID other than ">" reads entries already pending for this
		// consumer. Reaching the end resets the cursor so failures that
		// happened after startup are revisited on the next scan.
		xms, err = c.readGroup(readCtx, r, consumer, r.PendingID)
		if err != nil {
			return nil, fmt.Errorf("read pending: %w", err)
		}
		if len(xms) != 0 {
			r.PendingID = xms[len(xms)-1].ID
			r.readNewNext = true
			return collections.Map(xms, fromRedisMsg), nil
		}
		r.PendingID = r.pendingStartID
	}

	xms, err = c.readGroup(readCtx, r, consumer, ">")
	if err != nil {
		return nil, fmt.Errorf("read new: %w", err)
	}
	return collections.Map(xms, fromRedisMsg), nil
}

func (c *Consumer) readGroup(
	ctx context.Context,
	r *Route,
	consumer string,
	id string,
) ([]redis.XMessage, error) {
	xss, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    r.Group,
		Consumer: consumer,
		Streams:  []string{r.Stream, id},
		Count:    r.BatchSize,
		Block:    -1,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(xss) == 0 {
		return nil, nil
	}
	return xss[0].Messages, nil
}

// trim trims the streams to the max length.
func (c *Consumer) trim() {
	groups := lo.GroupBy(c.routes, func(it *Route) string { return it.Stream })
	trims := lo.MapEntries(groups, func(stream string, routes []*Route) (string, int64) {
		lens := collections.Map(routes, func(it *Route) int64 { return it.MaxLen })
		return stream, lo.Max(lens)
	})

	ctx := c.ctx
	logger := c.logger.With("task", "trim")
	run := func() {
		trimCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		for stream, maxLen := range trims {
			stats, err := c.trimStreamWithStats(trimCtx, stream, maxLen)
			streamLogger := logger.With(
				"stream", stream,
				"length", stats.length,
				"maxLen", maxLen,
				"groups", stats.groups,
				"pending", stats.pending,
				"lag", stats.lag,
				"safeBeforeID", stats.safeBeforeID,
				"trimmed", stats.trimmed,
			)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					streamLogger.ErrorContext(trimCtx, "trim error", "err", err)
				}
				continue
			}
			if stats.length > maxLen && stats.trimmed == 0 {
				streamLogger.WarnContext(
					trimCtx,
					"stream above max length; preserving entries required by consumer groups",
				)
			} else {
				streamLogger.DebugContext(trimCtx, "stream health checked")
			}
		}
	}

	run()
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

type trimStats struct {
	length       int64
	groups       int
	pending      int64
	lag          int64
	safeBeforeID string
	trimmed      int64
}

func (c *Consumer) trimStream(ctx context.Context, stream string, maxLen int64) error {
	_, err := c.trimStreamWithStats(ctx, stream, maxLen)
	return err
}

func (c *Consumer) trimStreamWithStats(
	ctx context.Context,
	stream string,
	maxLen int64,
) (trimStats, error) {
	var stats trimStats

	length, err := c.client.XLen(ctx, stream).Result()
	if err != nil {
		return stats, fmt.Errorf("get stream length: %w", err)
	}
	stats.length = length
	if length == 0 {
		return stats, nil
	}

	groups, err := c.client.XInfoGroups(ctx, stream).Result()
	if err != nil {
		return stats, fmt.Errorf("get consumer groups: %w", err)
	}
	stats.groups = len(groups)
	for _, group := range groups {
		stats.pending += group.Pending
		if group.Lag >= 0 {
			stats.lag += group.Lag
		}
	}

	if length <= maxLen || len(groups) == 0 {
		return stats, nil
	}

	for _, group := range groups {
		safeBeforeID := group.LastDeliveredID
		if group.Pending > 0 {
			pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: stream,
				Group:  group.Name,
				Start:  "-",
				End:    "+",
				Count:  1,
			}).Result()
			if err != nil {
				return stats, fmt.Errorf("get pending for group %q: %w", group.Name, err)
			}
			if len(pending) != 0 {
				safeBeforeID = pending[0].ID
			}
		}

		if safeBeforeID == "" || safeBeforeID == "0-0" {
			stats.safeBeforeID = safeBeforeID
			return stats, nil
		}
		if stats.safeBeforeID == "" {
			stats.safeBeforeID = safeBeforeID
			continue
		}
		before, err := streamIDBefore(safeBeforeID, stats.safeBeforeID)
		if err != nil {
			return stats, err
		}
		if before {
			stats.safeBeforeID = safeBeforeID
		}
	}

	if stats.safeBeforeID == "" {
		return stats, nil
	}
	stats.trimmed, err = c.client.XTrimMinID(ctx, stream, stats.safeBeforeID).Result()
	if err != nil {
		return stats, fmt.Errorf("trim before %q: %w", stats.safeBeforeID, err)
	}
	return stats, nil
}

func streamIDBefore(a, b string) (bool, error) {
	aTime, aSequence, err := parseStreamID(a)
	if err != nil {
		return false, err
	}
	bTime, bSequence, err := parseStreamID(b)
	if err != nil {
		return false, err
	}
	if aTime != bTime {
		return aTime < bTime, nil
	}
	return aSequence < bSequence, nil
}

func parseStreamID(id string) (uint64, uint64, error) {
	timePart, sequencePart, ok := strings.Cut(id, "-")
	if !ok {
		return 0, 0, fmt.Errorf("invalid Redis stream ID %q", id)
	}
	timeValue, err := strconv.ParseUint(timePart, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse Redis stream ID %q time: %w", id, err)
	}
	sequenceValue, err := strconv.ParseUint(sequencePart, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse Redis stream ID %q sequence: %w", id, err)
	}
	return timeValue, sequenceValue, nil
}
