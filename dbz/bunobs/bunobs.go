// Package bunobs configures Bun query logging and telemetry.
package bunobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/extra/bunotel"
	"github.com/uptrace/bun/extra/bunslog"
)

const (
	Local = "local"
	Dev   = "dev"
)

type options struct {
	queryText   bool
	logger      *slog.Logger
	otelOptions []bunotel.Option
}

// Option configures Bun observability.
type Option func(*options)

// WithLogger sends structured query logs to logger.
// Configure uses slog.Default when logger is nil or this option is omitted.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithOTelOptions configures the Bun OpenTelemetry query hook.
// Multiple calls append options in order.
func WithOTelOptions(opts ...bunotel.Option) Option {
	return func(o *options) {
		o.otelOptions = append(o.otelOptions, opts...)
	}
}

// WithQueryText allows full SQL in structured query logs when enabled.
// The logger's level still determines which query events are emitted.
func WithQueryText(enabled bool) Option {
	return func(o *options) {
		o.queryText = enabled
	}
}

// Configure attaches the query logger selected by env and Bun telemetry.
// Full SQL is visible locally and in dev when DEBUG logging is enabled.
// Other environments omit the query attribute unless WithQueryText enables it.
func Configure(db *bun.DB, env string, opts ...Option) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}

	if env == Local {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	} else {
		includeQuery := o.queryText
		if env == Dev && o.logger.Enabled(context.Background(), slog.LevelDebug) {
			includeQuery = true
		}
		db.AddQueryHook(bunslog.NewQueryHook(
			bunslog.WithLogger(o.logger),
			bunslog.WithLogFormat(func(event *bun.QueryEvent) []slog.Attr {
				return formatQueryLog(event, includeQuery)
			}),
		))
	}
	db.AddQueryHook(bunotel.NewQueryHook(o.otelOptions...))
}

func formatQueryLog(event *bun.QueryEvent, includeQuery bool) []slog.Attr {
	attrs := []slog.Attr{
		slog.Any("error", event.Err),
		slog.String("operation", event.Operation()),
		slog.String("duration", time.Since(event.StartTime).String()),
	}
	if includeQuery {
		attrs = append(attrs, slog.String("query", event.Query))
	}
	return attrs
}
