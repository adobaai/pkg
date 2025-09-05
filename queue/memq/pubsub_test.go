package memq

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adobaai/pkg/queue"
)

type Log struct {
	ID     int
	Status int
}

func getKey(it *Log) int {
	return it.ID
}

func assertNoEvent[K comparable, E any](t *testing.T, sub queue.Subscription[E]) {
	select {
	case <-sub.Ch():
		t.Fatal("sub received message")
	case <-time.After(50 * time.Millisecond):
		// Expected: no message
	}
}

func requireLength[K comparable, E any](
	t *testing.T, sub queue.Subscription[E], length int,
) []E {
	res := make([]E, 0)
	for range length {
		select {
		case msg := <-sub.Ch():
			res = append(res, msg)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("subdid not receive message in time")
		}
	}
	return res
}

func TestPubSubContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pb := NewPubSub(getKey)
	go func() {
		err := pb.Start(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	}()

	sub, err := pb.Sub(ctx, 1)
	require.NoError(t, err)

	require.NoError(t, pb.Pub(ctx, &Log{1, 10}))
	select {
	case msg := <-sub.Ch():
		require.Equal(t, 10, msg.Status)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sub did not receive message in time")
	}

	cancel()                          // Cancel the context for PubSub.Start
	time.Sleep(10 * time.Millisecond) // Give Start goroutine time to exit and Stop to be called

	err = pb.Pub(context.Background(), &Log{1, 20})
	require.ErrorIs(t, err, queue.ErrStopped)

	_, err = pb.Sub(context.Background(), 2)
	require.ErrorIs(t, err, queue.ErrStopped)

	assertNoEvent[int](t, sub)
	require.NoError(t, sub.Close())
	require.NoError(t, pb.Stop(context.Background()))
}

func TestPubSub(t *testing.T) {
	var (
		wg  sync.WaitGroup
		sub queue.Subscription[*Log]

		ctx    = context.Background()
		pubCap = 5
		subCap = 2
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		pb     = NewPubSub(
			getKey,
			WithLogger(logger),
			WithPubCapacity(uint(pubCap)),
			WithSubCapacity(uint(subCap)),
		)
	)

	t.Run("NotStarted", func(t *testing.T) {
		// Publish more messages than cap capacity
		for i := 0; i < pubCap+3; i++ {
			err := pb.Pub(ctx, &Log{1, i})
			if i < int(pubCap) {
				assert.NoError(t, err, "i=%d", i)
			} else {
				assert.ErrorIs(t, err, queue.ErrFull, "i=%d", i)
			}
		}

		// Wait a bit to ensure no unexpected subscribers pick it up
		time.Sleep(50 * time.Millisecond)
	})

	wg.Add(1)
	go func() {
		err := pb.Start(ctx)
		assert.NoError(t, err)
		wg.Done()
	}()

	time.Sleep(10 * time.Millisecond) // Wait for events to be dropped

	t.Run("PubCanceled", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		err := pb.Pub(ctx2, &Log{1, 10})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("NoSubscribers", func(t *testing.T) {
		require.NoError(t, pb.Pub(ctx, &Log{1, 10}))
		time.Sleep(10 * time.Millisecond) // Wait for event to be dropped

		// Now subscribe and publish another message
		var err error
		sub, err = pb.Sub(ctx, 1)
		require.NoError(t, err)

		require.NoError(t, pb.Pub(ctx, &Log{1, 20}))

		select {
		case msg := <-sub.Ch():
			require.Equal(t, 20, msg.Status)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub did not receive message in time")
		}

		require.NoError(t, sub.Close())
	})

	t.Run("Capacity", func(t *testing.T) {
		sub, err := pb.Sub(ctx, 1)
		require.NoError(t, err)

		for i := range subCap + 2 {
			err := pb.Pub(ctx, &Log{1, i + 10})
			assert.NoError(t, err, "i=%d", i)
		}

		// Wait for events to be dropped
		time.Sleep(10 * time.Millisecond)

		receivedCount := 0
		for i := 0; i < subCap+3; i++ {
			select {
			case <-sub.Ch():
				receivedCount++
				time.Sleep(10 * time.Millisecond)
			default:
			}
		}
		require.Equal(t, subCap, receivedCount)

		assertNoEvent[int](t, sub)
		require.NoError(t, sub.Close())
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		sub1, err := pb.Sub(ctx, 100)
		require.NoError(t, err)
		sub2, err := pb.Sub(ctx, 100)
		require.NoError(t, err)

		require.NoError(t, pb.Pub(ctx, &Log{100, 1}))
		require.NoError(t, pb.Pub(ctx, &Log{100, 2}))

		// Verify sub1 receives messages
		received1 := requireLength[int](t, sub1, 2)
		require.Len(t, received1, 2)
		require.Equal(t, 1, received1[0].Status)
		require.Equal(t, 2, received1[1].Status)

		// Verify sub2 receives messages
		received2 := requireLength[int](t, sub2, 2)
		require.Len(t, received2, 2)
		require.Equal(t, 1, received2[0].Status)
		require.Equal(t, 2, received2[1].Status)

		require.NoError(t, sub1.Close())
		require.NoError(t, sub2.Close())
	})

	t.Run("DifferentKeys", func(t *testing.T) {
		sub1, err := pb.Sub(ctx, 1)
		require.NoError(t, err)
		sub2, err := pb.Sub(ctx, 2)
		require.NoError(t, err)

		require.NoError(t, pb.Pub(ctx, &Log{1, 10}))
		require.NoError(t, pb.Pub(ctx, &Log{2, 20}))
		require.NoError(t, pb.Pub(ctx, &Log{1, 11}))

		// Verify sub1 receives messages for key 1
		received1 := requireLength[int](t, sub1, 2)
		require.Len(t, received1, 2)
		require.Equal(t, 10, received1[0].Status)
		require.Equal(t, 11, received1[1].Status)

		// Verify sub2 receives messages for key 2
		received2 := requireLength[int](t, sub2, 1)
		require.Len(t, received2, 1)
		require.Equal(t, 20, received2[0].Status)

		// Ensure sub1 does not receive messages for key 2
		select {
		case <-sub1.Ch():
			t.Fatal("sub1 received message for key 2")
		case <-time.After(50 * time.Millisecond):
			// Expected: no message
		}

		require.NoError(t, sub1.Close())
		require.NoError(t, sub2.Close())
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		sub1, err := pb.Sub(ctx, 1)
		require.NoError(t, err)
		sub2, err := pb.Sub(ctx, 1)
		require.NoError(t, err)

		require.NoError(t, pb.Pub(ctx, &Log{1, 10}))

		// Verify sub1 receives message
		select {
		case msg := <-sub1.Ch():
			require.Equal(t, 10, msg.Status)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub1 did not receive message in time")
		}

		// Verify sub2 receives message
		select {
		case msg := <-sub2.Ch():
			require.Equal(t, 10, msg.Status)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub2 did not receive message in time")
		}

		require.NoError(t, sub1.Close())

		require.NoError(t, pb.Pub(ctx, &Log{1, 20}))
		select {
		case <-sub1.Ch():
			t.Fatal("sub1 received message after closing")
		case <-time.After(50 * time.Millisecond):
			// Expected: no message
		}

		select {
		case msg := <-sub2.Ch():
			require.Equal(t, 20, msg.Status)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sub2 did not receive message in time after sub1 closed")
		}

		require.NoError(t, sub2.Close())
	})

	t.Run("Stop", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		require.ErrorIs(t, pb.Stop(ctx2), context.Canceled)

		require.NoError(t, pb.Stop(ctx))
		wg.Wait()

		err := pb.Pub(ctx, &Log{1, 20})
		require.ErrorIs(t, err, queue.ErrStopped)

		_, err = pb.Sub(ctx, 2)
		require.ErrorIs(t, err, queue.ErrStopped)

		err = pb.Start(context.Background())
		require.ErrorIs(t, err, queue.ErrStopped)

		assertNoEvent[int](t, sub)
	})
}
