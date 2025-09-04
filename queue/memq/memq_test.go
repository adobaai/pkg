package memq

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/adobaai/pkg/queue"
)

type Log struct {
	ID     int
	Status int
}

func TestMemory(t *testing.T) {
	ctx := context.Background()
	pb := NewPubSub(WithGetKey(func(it *Log) int { return it.ID }))
	require.NoError(t, pb.Pub(ctx, &Log{99, -9}))
	go func() {
		if err := pb.Start(ctx); err != queue.ErrStopped {
			t.Log("bad err:", err)
		}
	}()
	time.Sleep(time.Millisecond) // Wait for pb to start

	require.NoError(t, pb.Pub(ctx, &Log{99, -3}))
	sub, err := pb.Sub(ctx, 99)
	require.NoError(t, err)

	go func() {
		require.NoError(t, pb.Pub(ctx, &Log{99, -1}))
		require.NoError(t, pb.Pub(ctx, &Log{99, 1}))
		require.NoError(t, pb.Pub(ctx, &Log{99, 9}))
	}()

	for v := range sub.Ch() {
		if v.Status == 1 {
			break
		}
		t.Log(v)
	}
	t.Log("breaked")
	require.NoError(t, sub.Cancel())
}
