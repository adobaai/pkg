// bunobs-smoke exercises Bun query logging and telemetry against in-memory SQLite.
// Run with -env local|dev|prod, -level DEBUG|INFO, and optional -query-text.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bunotel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/adobaai/pkg/dbz/bunobs"
)

const sampleValue = "bunobs-sample-value"

func main() {
	env := flag.String("env", bunobs.Local, "environment: local, dev, or prod")
	levelName := flag.String("level", "INFO", "structured log level")
	queryText := flag.Bool("query-text", false, "include SQL text in non-local structured logs")
	defaultLogger := flag.Bool("default-logger", false, "use slog.Default instead of WithLogger")
	flag.Parse()

	var level slog.Level
	if err := level.UnmarshalText([]byte(*levelName)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := run(context.Background(), *env, level, *queryText, *defaultLogger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	ctx context.Context, env string, level slog.Level, queryText, defaultLogger bool,
) (err error) {
	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	if err != nil {
		return err
	}
	db := bun.NewDB(sqldb, sqlitedialect.New())

	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	metricReader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	defer func() {
		err = errors.Join(err, db.Close(), tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	opts := []bunobs.Option{
		bunobs.WithQueryText(queryText),
		bunobs.WithOTelOptions(
			bunotel.WithDBName("bunobs-smoke"),
			bunotel.WithTracerProvider(tracerProvider),
			bunotel.WithMeterProvider(meterProvider),
		),
	}
	if defaultLogger {
		slog.SetDefault(logger)
	} else {
		opts = append(opts, bunobs.WithLogger(logger))
	}
	bunobs.Configure(db, env, opts...)

	fmt.Printf("environment=%s level=%s query-text=%t\n", env, level, queryText)
	var value string
	if err := db.NewRaw("SELECT ?", sampleValue).Scan(ctx, &value); err != nil {
		return fmt.Errorf("successful query: %w", err)
	}
	if value != sampleValue {
		return fmt.Errorf("unexpected query result: %q", value)
	}
	fmt.Println("successful query: OK")

	err = db.NewRaw("SELECT ? WHERE 1 = 0", sampleValue).Scan(ctx, &value)
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("no-row query: expected sql.ErrNoRows, got %v", err)
	}
	fmt.Println("no-row query: sql.ErrNoRows")

	err = db.
		NewRaw("SELECT * FROM missing_bunobs_table WHERE marker = ?", sampleValue).
		Scan(ctx, &value)
	if err == nil {
		return errors.New("error query: expected a database error")
	}
	fmt.Printf("error query: %v\n", err)

	gotSpans := spans.GetSpans()
	if len(gotSpans) != 3 {
		return fmt.Errorf("expected 3 OTel spans, got %d", len(gotSpans))
	}
	for _, span := range gotSpans {
		var databaseName, statement string
		for _, attr := range span.Attributes {
			switch attr.Key {
			case "db.name":
				databaseName = attr.Value.AsString()
			case "db.statement":
				statement = attr.Value.AsString()
			}
		}
		if databaseName != "bunobs-smoke" {
			return fmt.Errorf("span %q has db.name=%q", span.Name, databaseName)
		}
		fmt.Printf("span: %s db.name=%q statement=%q\n", span.Name, databaseName, statement)
	}

	var metrics metricdata.ResourceMetrics
	if err := metricReader.Collect(ctx, &metrics); err != nil {
		return fmt.Errorf("collect OTel metrics: %w", err)
	}
	foundTiming := false
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			fmt.Printf("metric: %s\n", metric.Name)
			if metric.Name == "go.sql.query_timing" {
				foundTiming = true
			}
		}
	}
	if !foundTiming {
		return errors.New("missing go.sql.query_timing metric")
	}
	return nil
}
