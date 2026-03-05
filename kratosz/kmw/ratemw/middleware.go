package ratemw

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"golang.org/x/time/rate"
)

const (
	rlTokenCookie = "_rl_token"
)

type UserIDExtractor func(ctx context.Context) (string, bool)

func Middleware(stores *StrategyStores, extractor UserIDExtractor) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			resolved, err := resolveKey(ctx, httpRequestFromContext(ctx), extractor)
			if err != nil {
				return nil, err
			}
			limiter := stores.Get(resolved)
			if !limiter.Allow() {
				setRateLimitHeaders(ctx, limiter, stores.LimitFor(resolved.Strategy))
				return nil, WithRetryAfter(ErrRPMLimitExceeded, retryAfterSeconds(limiter))
			}

			return handler(ctx, req)
		}
	}
}

func WaitMiddleware(stores *StrategyStores, extractor UserIDExtractor) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			resolved, err := resolveKey(ctx, httpRequestFromContext(ctx), extractor)
			if err != nil {
				return nil, err
			}
			limiter := stores.Get(resolved)
			if err := limiter.Wait(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, err
				}
				setRateLimitHeaders(ctx, limiter, stores.LimitFor(resolved.Strategy))
				return nil, WithRetryAfter(ErrRPMLimitExceeded, 1)
			}

			return handler(ctx, req)
		}
	}
}

func resolveKey(ctx context.Context, req *http.Request, extractor UserIDExtractor) (ResolvedKey, error) {
	if extractor == nil {
		return ResolvedKey{}, ErrInvalidUserIDExtractor
	}

	if userID, ok := extractor(ctx); ok {
		return ResolvedKey{
			Key:      "auth:" + userID,
			Strategy: StrategyAuth,
		}, nil
	}

	if req == nil {
		return ResolvedKey{
			Key:      "anon:ip:unknown",
			Strategy: StrategyIP,
		}, nil
	}

	if cookie, err := req.Cookie(rlTokenCookie); err == nil && cookie != nil && cookie.Value != "" {
		return ResolvedKey{
			Key:      "anon:cookie:" + cookie.Value,
			Strategy: StrategyCookie,
		}, nil
	}

	ip := extractIP(req)
	if ip == "" {
		ip = "unknown"
	}
	return ResolvedKey{
		Key:      "anon:ip:" + ip,
		Strategy: StrategyIP,
	}, nil
}

func httpRequestFromContext(ctx context.Context) *http.Request {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return nil
	}
	ht, ok := tr.(khttp.Transporter)
	if !ok {
		return nil
	}
	return ht.Request()
}

func extractIP(req *http.Request) string {
	if req == nil {
		return ""
	}

	if xri := strings.TrimSpace(req.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(req.RemoteAddr)
}

// setRateLimitHeaders sets standard rate limit headers on the HTTP response
func setRateLimitHeaders(ctx context.Context, limiter *rate.Limiter, limit Options) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return
	}

	header := tr.ReplyHeader()
	if header == nil {
		return
	}

	// Retry-After header (seconds to wait)
	retryAfter := retryAfterSeconds(limiter)
	header.Set("Retry-After", strconv.Itoa(retryAfter))

	// X-RateLimit headers
	header.Set("X-RateLimit-Limit", strconv.Itoa(limit.Burst))

	// Approximate remaining requests (this is an estimate)
	tokens := limiter.Tokens()
	remaining := int(math.Max(0, tokens))
	header.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

	// Note: The APISIX `limit-count` plugin does not provide `X-RateLimit-Reset` header,
	// so we won't set it here.

	// Reset time (approximate Unix timestamp when limit resets)
	// resetTime := time.Now().Add(time.Duration(retryAfter) * time.Second).Unix()
	// header.Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
}

// retryAfterSeconds calculates the number of seconds a client should wait
// before retrying a rate-limited request, based on the limiter's current state.
// It makes a reservation to determine how long until the next token is available.
func retryAfterSeconds(limiter *rate.Limiter) int {
	reservation := limiter.Reserve()
	if !reservation.OK() {
		return 1
	}
	delay := reservation.Delay()
	reservation.CancelAt(time.Now())

	if delay <= 0 || delay == rate.InfDuration {
		return 1
	}
	sec := int(math.Ceil(delay.Seconds()))
	if sec < 1 {
		return 1
	}
	return sec
}
