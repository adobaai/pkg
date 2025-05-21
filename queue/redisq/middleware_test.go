package redisq

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Closure() Middleware {
	fmt.Println("it called 1")
	return func(h Handler) Handler {
		fmt.Println("it called 2")
		return func(ctx Context) (err error) {
			return h(ctx)
		}
	}
}

func TestRecover(t *testing.T) {
	l := slog.Default()
	m := Chain(Recover(l))
	h := func(Context) error {
		panic("test panic")
	}
	h = m(h)
	r := Route{
		Stream: stream,
		Group:  group,
	}
	ctx := newContext(context.Background(), &r, RM{})
	assert.ErrorIs(t, h(ctx), Panicked)
}

func TestTracing(t *testing.T) {
	ackIDs := []string{"hello", "world"}
	l := slog.Default()
	m := Chain(Recover(l), Tracing())
	h := func(ctx Context) error {
		ctx.Ack(ackIDs...)
		return nil
	}
	h = m(h)
	r := Route{
		Stream: stream,
		Group:  group,
	}
	ctx := newContext(context.Background(), &r, RM{})
	require.NoError(t, h(ctx))
	assert.Equal(t, ackIDs, ctx.getAckIDs())
}

func TestClosure(t *testing.T) {
	l := slog.Default()
	h := func(Context) error {
		t.Log("handling")
		return nil
	}
	m := Chain(Recover(l), Closure())
	m(h)
	m(h)
	r := Route{
		Stream: stream,
		Group:  group,
	}
	ctx := newContext(context.Background(), &r, RM{})
	assert.NoError(t, h(ctx))
}
