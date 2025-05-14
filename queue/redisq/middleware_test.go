package redisq

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
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
	ctx := NewContext(context.Background(), &r, M2{})
	assert.NoError(t, h(ctx))
}
