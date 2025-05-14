package redisq

import (
	"context"
	"testing"
	"time"

	"github.com/adobaai/pkg/testingz"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	stream = "test:stream"
	group  = "test-group"
)

const testKeyPrefix = "ut:pkg:redisq:"

func TestXReadPending(t *testing.T) {
	ctx := context.Background()
	stream := "test:stream"
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
			Values: map[string]any{"init": "1"},
		}).Err,

		rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err,
	)
	require.NoError(t, err)

	// Clear initial message so we block on read
	xread := redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    -1,
	}

	xss := testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss, 1)
	go func() {
		time.Sleep(5 * time.Second)
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"init": "1"},
		})
	}()
	xss = testingz.R(rdb.XReadGroup(ctx, &xread).Result()).NoError(t).V()
	t.Log(xss, 2)

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

	testingz.R(rdb.Get(ctx, key).Result()).NoError(t).Log()
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
