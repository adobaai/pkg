package cronz

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Recovery creates a middleware that recovers from panics.
func Recovery(logger *slog.Logger) Middleware {
	logger = logger.With("component", "cronz")
	return func(next Action) Action {
		return func(ctx Context) error {
			defer func() {
				if r := recover(); r != nil {
					logger.ErrorContext(ctx, "cron job panicked", "panic", r)
				}
			}()

			return next(ctx)
		}
	}
}

// Logging creates a middleware that adds detailed logging.
func Logging(logger *slog.Logger) Middleware {
	logger = logger.With("component", "cronz")
	return func(next Action) Action {
		return func(ctx Context) error {
			start := time.Now()
			logger.With("name", ctx.Name())
			logger.InfoContext(ctx, "cron job executing", "start", start)

			err := next(ctx)

			duration := time.Since(start)
			if err != nil {
				logger.ErrorContext(ctx, "cron job failed",
					"duration", duration,
					"error", err)
			} else {
				logger.InfoContext(ctx, "cron job completed",
					"duration", duration)
			}

			return err
		}
	}
}

// Trace creates a middleware that adds OpenTelemetry tracing.
func Trace(tracerName string) Middleware {
	tracer := otel.Tracer("github.com/adobaai/pkg/cronz")
	return func(next Action) Action {
		return func(ctx Context) error {
			baseCtx, span := tracer.Start(ctx,
				"[cronz] run "+ctx.Name(),
				trace.WithSpanKind(trace.SpanKindConsumer),
			)
			defer span.End()

			ctx = ctx.WithContext(baseCtx)
			err := next(ctx)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return err
		}
	}
}

// Timeout creates a middleware that adds a timeout to job execution.
func Timeout(timeout time.Duration) Middleware {
	return func(next Action) Action {
		return func(ctx Context) error {
			baseCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			ctx = ctx.WithContext(baseCtx)
			done := make(chan error, 1)
			go func() {
				done <- next(ctx)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
