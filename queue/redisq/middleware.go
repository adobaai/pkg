package redisq

import (
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Handler func(ctx Context) error

type Middleware func(Handler) Handler

// Chain creates a single [Middleware] by chaining multiple [Middleware] functions.
func Chain(m ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(m) - 1; i >= 0; i-- {
			next = m[i](next)
		}
		return next
	}
}

func Recover(l *slog.Logger) Middleware {
	return func(h Handler) Handler {
		return func(ctx Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					route := ctx.Route()
					l.ErrorContext(ctx, "recover", "route", route.SpanName(), "value", r)
				}
			}()
			return h(ctx)
		}
	}
}

func Tracing() Middleware {
	tracer := otel.Tracer("github.com/adobaai/pkg/queue/redisq")
	return func(h Handler) Handler {
		return func(ctx Context) (err error) {
			r := ctx.Route()
			traceCtx, span := tracer.Start(
				ctx,
				r.SpanName(),
				trace.WithSpanKind(trace.SpanKindConsumer),
			)
			if err := h(ctx.WithContext(traceCtx)); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
			return err
		}
	}
}
