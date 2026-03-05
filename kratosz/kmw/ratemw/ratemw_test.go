package ratemw

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

var (
	// Common test options to avoid repetition
	testOptsBasic  = Options{Rate: 1, Burst: 1, StaleAfter: time.Hour, MaxEntries: 10}
	testOptsActive = Options{Rate: 1, Burst: 1, StaleAfter: 5 * time.Minute, MaxEntries: 10}
	testOptsMemory = Options{Rate: 1, Burst: 1, StaleAfter: time.Hour, MaxEntries: 2_000}
	testOptsMaxCap = Options{Rate: 1, Burst: 1, StaleAfter: time.Hour, MaxEntries: 1}
)

func TestStore_Get(t *testing.T) {
	t.Run("NewUser", func(t *testing.T) {
		s := NewStore(testOptsBasic)
		t.Cleanup(s.Close)

		limiter := s.Get("u1")
		require.NotNil(t, limiter)
		assert.Equal(t, 1, s.Len())
	})

	t.Run("SameUser", func(t *testing.T) {
		s := NewStore(testOptsBasic)
		t.Cleanup(s.Close)

		first := s.Get("u1")
		second := s.Get("u1")
		require.NotNil(t, first)
		require.NotNil(t, second)
		assert.Same(t, first, second)
	})

	// MaxEntriesEviction tests that the cache remains bounded and continues
	// serving new users when capacity pressure is reached.
	t.Run("MaxEntriesEviction", func(t *testing.T) {
		s := NewStore(testOptsMaxCap)
		t.Cleanup(s.Close)

		first := s.Get("u1")
		assert.True(t, first.Allow())
		assert.Equal(t, 1, s.Len())

		second := s.Get("u2")
		assert.True(t, second.Allow())

		require.Eventually(t, func() bool {
			s.cache.CleanUp()
			return s.Len() <= 1
		}, time.Second, 10*time.Millisecond)

		third := s.Get("u3")
		assert.NotNil(t, third)
	})
}

func TestStore_Evict(t *testing.T) {
	t.Run("StaleEntry", func(t *testing.T) {
		s := NewStore(Options{Rate: 1, Burst: 1, StaleAfter: 10 * time.Millisecond, MaxEntries: 10})
		t.Cleanup(s.Close)

		first := s.Get("u1")
		time.Sleep(25 * time.Millisecond)
		s.cache.CleanUp()
		assert.Equal(t, 0, s.Len())

		second := s.Get("u1")

		assert.NotSame(t, first, second)
		assert.Equal(t, 1, s.Len())
	})

	t.Run("ActiveEntry", func(t *testing.T) {
		s := NewStore(testOptsActive)
		t.Cleanup(s.Close)

		first := s.Get("u1")
		s.cache.CleanUp()
		second := s.Get("u1")

		assert.Same(t, first, second)
		assert.Equal(t, 1, s.Len())
	})
}

func TestStore_Memory(t *testing.T) {
	t.Run("Estimate", func(t *testing.T) {
		s := NewStore(testOptsMemory)
		t.Cleanup(s.Close)

		for i := range 1000 {
			id := strconv.Itoa(i)
			s.cache.Set(id, rate.NewLimiter(1, 1))
		}

		assert.Equal(t, 1000*bytesPerRecord, s.EstimateMemoryBytes())
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("NilExtractor", func(t *testing.T) {
		stores := NewStrategyStores(DefaultOptions())
		t.Cleanup(stores.Close)

		mw := Middleware(stores, nil)
		h := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		_, err := h(context.Background(), nil)
		require.Error(t, err)
		assert.True(t, ErrInvalidUserIDExtractor.Is(err))
	})

	t.Run("Allow", func(t *testing.T) {
		stores := NewStrategyStoresWithLimits(DefaultOptions(), map[KeyStrategy]Options{
			StrategyAuth: {Rate: rate.Inf, Burst: 1},
		})
		t.Cleanup(stores.Close)

		mw := Middleware(stores, testExtractor)
		hit := false
		h := mw(func(ctx context.Context, req any) (any, error) {
			hit = true
			return "ok", nil
		})

		res, err := h(withClaims(1), nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", res)
		assert.True(t, hit)
	})

	t.Run("Deny", func(t *testing.T) {
		stores := NewStrategyStoresWithLimits(DefaultOptions(), map[KeyStrategy]Options{
			StrategyAuth: {Rate: 0, Burst: 0},
		})
		t.Cleanup(stores.Close)

		mw := Middleware(stores, testExtractor)
		h := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		_, err := h(withClaims(1), nil)
		require.Error(t, err)
		assert.Equal(t, 429, int(kerrors.FromError(err).Code))

		se := kerrors.FromError(err)
		assert.NotEmpty(t, se.Metadata["retry_after"])
	})

	t.Run("AnonymousUser", func(t *testing.T) {
		stores := NewStrategyStores(DefaultOptions())
		t.Cleanup(stores.Close)

		mw := Middleware(stores, testExtractor)
		h := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		res, err := h(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", res)
	})

	t.Run("ContextCancel", func(t *testing.T) {
		stores := NewStrategyStoresWithLimits(DefaultOptions(), map[KeyStrategy]Options{
			StrategyAuth: {Rate: 1, Burst: 1},
		})
		t.Cleanup(stores.Close)

		mw := WaitMiddleware(stores, testExtractor)
		h := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		ctx, _ := context.WithDeadline(withClaims(1), time.Now().Add(-time.Second))

		_, err := h(ctx, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}

func withClaims(uid int64) context.Context {
	return context.WithValue(context.Background(), "test_uid", uid)
}

func testExtractor(ctx context.Context) (string, bool) {
	if uid, ok := ctx.Value("test_uid").(int64); ok && uid > 0 {
		return strconv.FormatInt(uid, 10), true
	}
	return "", false
}

func TestUtilities(t *testing.T) {
	t.Run("WithRetryAfter", func(t *testing.T) {
		err := WithRetryAfter(ErrRPMLimitExceeded, 3)
		require.Error(t, err)
		assert.Equal(t, 429, int(kerrors.FromError(err).Code))
		se := kerrors.FromError(err)
		assert.Equal(t, "3", se.Metadata["retry_after"])
	})

	t.Run("MiddlewareHandlerType", func(t *testing.T) {
		stores := NewStrategyStores(DefaultOptions())
		t.Cleanup(stores.Close)
		var _ middleware.Middleware = Middleware(stores, testExtractor)
		var _ middleware.Middleware = WaitMiddleware(stores, testExtractor)
	})
}

func TestResolveKey(t *testing.T) {
	t.Run("NilExtractor", func(t *testing.T) {
		_, err := resolveKey(context.Background(), nil, nil)
		require.Error(t, err)
		assert.True(t, ErrInvalidUserIDExtractor.Is(err))
	})

	t.Run("Auth", func(t *testing.T) {
		resolved, err := resolveKey(withClaims(42), nil, testExtractor)
		require.NoError(t, err)
		assert.Equal(t, StrategyAuth, resolved.Strategy)
		assert.Equal(t, "auth:42", resolved.Key)
	})

	t.Run("Cookie", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: rlTokenCookie, Value: "cookie-token"})

		resolved, err := resolveKey(context.Background(), req, testExtractor)
		require.NoError(t, err)
		assert.Equal(t, StrategyCookie, resolved.Strategy)
		assert.Equal(t, "anon:cookie:cookie-token", resolved.Key)
	})

	t.Run("IPFallback", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
		require.NoError(t, err)
		req.Header.Set("X-Real-IP", "203.0.113.4")

		resolved, err := resolveKey(context.Background(), req, testExtractor)
		require.NoError(t, err)
		assert.Equal(t, StrategyIP, resolved.Strategy)
		assert.Equal(t, "anon:ip:203.0.113.4", resolved.Key)
	})

	t.Run("UseXRealIPOnly", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-For", "198.51.100.77")
		req.Header.Set("X-Real-IP", "203.0.113.4")

		resolved, err := resolveKey(context.Background(), req, testExtractor)
		require.NoError(t, err)
		assert.Equal(t, StrategyIP, resolved.Strategy)
		assert.Equal(t, "anon:ip:203.0.113.4", resolved.Key)
	})
}

func TestStrategyStores_LimitFor(t *testing.T) {
	base := DefaultOptions()
	stores := NewStrategyStoresWithLimits(base, map[KeyStrategy]Options{
		StrategyAuth: {Rate: 7, Burst: 13},
		StrategyIP:   {Rate: 2, Burst: 3},
	})
	t.Cleanup(stores.Close)

	authLimit := stores.LimitFor(StrategyAuth)
	assert.Equal(t, 13, authLimit.Burst)
	assert.Equal(t, rate.Limit(7), authLimit.Rate)

	missingLimit := stores.LimitFor(StrategyCookie)
	assert.Equal(t, 3, missingLimit.Burst)
	assert.Equal(t, rate.Limit(2), missingLimit.Rate)
}
