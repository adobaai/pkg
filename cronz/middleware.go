package cronz

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/adobaai/pkg/middleware"
)

type Middleware = middleware.Middleware[Context]

// Recovery creates a middleware that recovers from panics.
func Recovery(logger *slog.Logger, stack bool) Middleware {
	logger = logger.With("component", "cronz")
	return func(next Action) Action {
		return func(ctx Context) error {
			defer func() {
				if r := recover(); r != nil {
					if stack {
						const size = 64 << 10 // 64KB
						buf := make([]byte, size)
						buf = buf[:runtime.Stack(buf, false)]
						// println(string(buf)) // For debugging
						logger.ErrorContext(ctx, "cron job panicked",
							"panic", r,
							"stack", string(buf))
					} else {
						logger.ErrorContext(ctx, "cron job panicked", "panic", r)
					}
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
			l := logger.With(
				"name", ctx.Name(),
				"run_id", ctx.RunID(),
			)
			l.InfoContext(ctx, "cron job executing", "start", start)

			err := next(ctx)

			duration := time.Since(start)
			if err != nil {
				l.ErrorContext(ctx, "cron job failed",
					"duration", duration,
					"error", err,
				)
			} else {
				l.InfoContext(ctx, "cron job completed", "duration", duration)
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
				trace.WithAttributes(
					attribute.String("job.name", ctx.Name()),
					attribute.String("job.run_id", ctx.RunID()),
				),
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

// Metrics creates a middleware that collects job execution metrics.
func Metrics() Middleware {
	meter := otel.Meter("github.com/adobaai/pkg/cronz")
	jobDuration, _ := meter.Float64Histogram(
		"cronz.job.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("Cron job execution duration in milliseconds"),
	)
	jobCount, _ := meter.Int64Counter(
		"cronz.job.count",
		metric.WithDescription("Count of cron job executions"),
	)

	return func(next Action) Action {
		return func(ctx Context) error {
			attrs := []attribute.KeyValue{
				attribute.String("job.name", ctx.Name()),
				attribute.String("job.run_id", ctx.RunID()),
			}
			start := time.Now()
			jobCount.Add(ctx, 1, metric.WithAttributes(attrs...))

			err := next(ctx)

			duration := time.Since(start)
			jobDuration.Record(ctx, float64(duration.Milliseconds()),
				metric.WithAttributes(attrs...),
				metric.WithAttributes(attribute.Bool("job.error", err != nil)),
			)

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
			done := make(chan error)
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
