package redisq

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adobaai/pkg/collections"
	"github.com/adobaai/pkg/testingz"
)

const (
	stream = "test:stream"
	group  = "test-group"
)

const testKeyPrefix = "ut:pkg:redisq:"

type Hook struct{}

func (Hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (Hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		fmt.Println(cmd.String())
		return next(ctx, cmd)
	}
}

func (Hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}

type UserEvent struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	EventType string `json:"event_type"`
}

func TestConsumer(t *testing.T) {
	var (
		l      = slog.Default()
		ctx    = context.Background()
		stream = testKeyPrefix + "consumer"
		rdb    = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	)

	rdb.AddHook(Hook{})

	err := ChainF(
		rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err,
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{
				"content": "You can now check out APISIX 3.0",
			},
		}).Err,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdb.Del(ctx, stream).Err())
	})

	t.Run("Recover", func(t *testing.T) {
		c := NewConsumer(rdb, l, WithMiddlewares(Recover(l)))
		h := func(ctx Context) error {
			panic("haha")
		}
		c.MustAddRoute(&Route{
			Stream:  stream,
			Group:   group,
			Handler: h,
		})
		consume(t, ctx, c, time.Second)
	})

	t.Run("Server", func(t *testing.T) {
		count := 0
		h := func(ctx Context) error {
			count++
			t.Logf("got message: \n%+v", ctx.Msg())
			return nil
		}
		c := NewConsumer(rdb, l)
		c.MustAddRoute(&Route{
			Stream:  stream,
			Group:   group,
			Handler: h,
		})
		consume(t, ctx, c, time.Second)
		assert.Equal(t, 1, count)
	})

	t.Run("Generic", func(t *testing.T) {
		stream := testKeyPrefix + "generic"
		err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rdb.Del(ctx, stream).Err())
		})

		ue := UserEvent{
			UserID:    "user_12345",
			Email:     "jane.doe@example.com",
			Username:  "janedoe",
			EventType: "UserRegistered",
		}
		m := NewM(ue)
		require.NoError(t, Publish(ctx, rdb, stream, m))

		c := NewConsumer(rdb, l)
		r := &Route{
			Stream: stream,
			Group:  group,
		}
		MustAddHandler(c, r, func(ctx Context, m *M[UserEvent]) error {
			assert.False(t, ctx.IsBatch())
			t.Log(m.T)
			return nil
		})

		consume(t, ctx, c, time.Second)
	})

	t.Run("Generic2", func(t *testing.T) {
		stream := testKeyPrefix + "generic2"
		err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rdb.Del(ctx, stream).Err())
		})

		for i := range 5 {
			ue := UserEvent{
				UserID:    "user_" + strconv.Itoa(i),
				Email:     "jane.doe@example.com",
				Username:  "janedoe",
				EventType: "UserRegistered",
			}
			m := NewM(ue)
			require.NoError(t, Publish(ctx, rdb, stream, m))
		}

		c := NewConsumer(rdb, l)
		r := &Route{
			Stream:    stream,
			Group:     group,
			BatchSize: 2,
		}
		MustAddBatchHandler(c, r, func(ctx Context, ms []*M[UserEvent]) error {
			assert.True(t, ctx.IsBatch())
			t.Log(ms)
			return nil
		})

		consume(t, ctx, c, time.Second)
	})

	t.Run("Batch", func(t *testing.T) {
		stream := testKeyPrefix + "batch"
		err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rdb.Del(ctx, stream).Err())
		})

		msgsCount := 10
		cmds, err := rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
			for range msgsCount {
				p.XAdd(ctx, &redis.XAddArgs{
					Stream: stream,
					Values: map[string]any{
						"content": "Time is " + time.Now().Format(time.RFC3339),
					},
				})
			}
			return nil
		})
		require.NoError(t, err)
		msgIDs := collections.Map(cmds, func(it redis.Cmder) string {
			return it.(*redis.StringCmd).Val()
		})

		okMsgIDs := lo.Subset(msgIDs, 0, 6)
		h := func(ctx Context) (err error) {
			msgs := ctx.Msgs()
			ids := collections.Map(msgs, getID)
			okIDs := lo.Intersect(ids, okMsgIDs)
			ctx.Ack(okIDs...)
			if len(okIDs) != len(msgs) {
				err = fmt.Errorf("len(okIDs) is %d, len(msgs) is %d", len(okIDs), len(msgs))
			}
			return
		}
		c := NewConsumer(rdb, l, WithMiddlewares(Tracing()))
		c.MustAddRoute(&Route{
			Stream:    stream,
			Group:     group,
			Handler:   h,
			BatchSize: 5,
		})
		consume(t, ctx, c, 2*time.Second)

		infoCmd := rdb.XInfoGroups(ctx, stream)
		require.NoError(t, infoCmd.Err())
		assert.Equal(t, group, infoCmd.Val()[0].Name)
		assert.Equal(t, int64(4), infoCmd.Val()[0].Pending)

		lenCmd := rdb.XLen(ctx, stream)
		require.NoError(t, lenCmd.Err())
		assert.Equal(t, int64(msgsCount), lenCmd.Val())
	})
}

func consume(t *testing.T, ctx context.Context, c *Consumer, wait time.Duration) {
	exit := make(chan struct{})
	go func() {
		err := c.Start(ctx)
		assert.NoError(t, err)
		close(exit)
	}()

	time.Sleep(wait)
	err := c.Stop(ctx)
	assert.NoError(t, err)
	<-exit
}

func TestXReadGroup(t *testing.T) {
	ctx := context.Background()
	stream := testKeyPrefix + "pending"
	group := "test-group"
	consumer := "test1"

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	defer rdb.Close()

	// Clean up
	rdb.Del(ctx, stream)
	rdb.XGroupDestroy(ctx, stream, group)

	err := ChainF(
		// Create stream and group
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"init": "1", "hello": "world"},
		}).Err,

		rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err,
	)
	require.NoError(t, err)

	// Clear initial message so we block on read
	xread := redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    3,
		Block:    -1,
	}

	xss := testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss, 1)
	go func() {
		time.Sleep(600 * time.Millisecond)
		cmd := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"init": "1"},
		})
		assert.NoError(t, cmd.Err())
	}()
	testingz.R(rdb.XReadGroup(ctx, &xread).Result()).ErrorIs(t, redis.Nil)

	time.Sleep(1 * time.Second)
	xss = testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss)

	xread.Streams = []string{stream, "0"}
	xss = testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss)

	xread.Streams = []string{stream, xss[0].Messages[0].ID}
	xss = testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss)

	start := time.Now()
	xread.Streams = []string{stream, xss[0].Messages[0].ID}
	xread.Block = time.Second
	xss = testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss)
	// [{ut:pkg:redisq:pending []}]
	assert.Less(t, time.Since(start), 200*time.Millisecond)
	// Without blocking
	//
	// Refer https://redis.io/docs/latest/commands/xreadgroup/:
	//   Note that in this case, both BLOCK and NOACK are ignored.

	start = time.Now()
	xread.Streams = []string{stream, ">"}
	testingz.R(rdb.XReadGroup(ctx, &xread).Result()).ErrorIs(t, redis.Nil)
	assert.Greater(t, time.Since(start), time.Second)
	// Blocked
}

func TestXReadGroupBlockingBlocksClient(t *testing.T) {
	ctx := context.Background()
	stream := "test:stream"
	group := "test-group"
	consumer := "test1"

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		PoolSize: 2,
	})

	defer rdb.Close()

	// Clean up
	rdb.Del(ctx, stream)
	rdb.XGroupDestroy(ctx, stream, group)

	err := ChainF(
		// Create stream and group
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"init": "1"},
		}).Err,

		rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err,

		// Clear initial message so we block on read
		rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    time.Second,
		}).Err,
	)
	require.NoError(t, err)

	blockingDone := make(chan struct{})

	go func() {
		// Start blocking XREADGROUP
		t.Log("Starting blocking XREADGROUP")
		_, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Block:    10 * time.Second,
			Count:    1,
		}).Result()

		t.Log("XREADGROUP done:", err)
		close(blockingDone)
	}()

	// Give the XREADGROUP a moment to block
	time.Sleep(1 * time.Second)

	// Try to PING using the same client (this should block)
	pingDone := make(chan struct{})

	go func() {
		t.Log("Trying to PING (should block)")
		start := time.Now()
		err := rdb.Ping(ctx).Err()
		elapsed := time.Since(start)
		t.Logf("PING done (after %v): %v\n", elapsed, err)
		close(pingDone)
	}()

	// Wait 2 seconds, then unblock the XREADGROUP by writing a message
	time.Sleep(2 * time.Second)
	t.Log("Writing message to unblock XREADGROUP")
	_ = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"unblock": "yes"},
	}).Err()

	// Wait for both goroutines to finish
	select {
	case <-blockingDone:
	case <-time.After(5 * time.Second):
		t.Fatal("XREADGROUP did not finish in time")
	}

	select {
	case <-pingDone:
	case <-time.After(5 * time.Second):
		t.Fatal("PING did not finish in time")
	}
}

func TestRedis(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	key := testKeyPrefix + "put-bytes"
	err := rdb.Set(ctx, key, []byte(key), 0).Err()
	assert.NoError(t, err)

	testingz.R(rdb.Get(ctx, key).Result()).NoError(t).Do(func(t *testing.T, it string) {
		t.Log(it)
	})
}

// ChainF executes a series of functions sequentially and returns the first error encountered.
//
// It is useful for chaining multiple operations where each operation returns an error.
// If all functions succeed, it returns nil.
func ChainF(fs ...func() error) error {
	for _, f := range fs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
