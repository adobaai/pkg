package cronz

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	s := New(WithLogger(logger), WithCronOptions(cron.WithSeconds()))
	s.Use(Recovery(logger, false), Logging(logger))

	// Register a job
	count := 0
	if err := s.Register("test-job", "* * * * * *", func(ctx Context) error {
		logger.Info("test job running", "name", ctx.Name(), "spec", ctx.Spec())
		count++
		return nil
	}); err != nil {
		t.Fatalf("failed to register cron job: %v", err)
	}

	startCh := make(chan error)
	go func() {
		startCh <- s.Start(ctx)
	}()

	// Wait for the job to run
	time.Sleep(2 * time.Second)
	logger.Info("test job ran successfully")
	assert.Equal(t, 2, count, "job should run twice")

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("failed to stop cron server: %v", err)
	}

	err := <-startCh
	assert.NoError(t, err, "cron server should start without error")
}

func TestRecovery(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	s := New(WithLogger(logger), WithCronOptions(cron.WithSeconds()))
	s.Use(Recovery(logger, true), Logging(logger))

	// Register a panicking job
	if err := s.Register("panic-job", "* * * * * *", func(ctx Context) error {
		panic("intentional panic for testing recovery")
	}); err != nil {
		t.Fatalf("failed to register cron job: %v", err)
	}

	startCh := make(chan error)
	go func() {
		startCh <- s.Start(ctx)
	}()

	// Wait for the job to run
	time.Sleep(2 * time.Second)

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("failed to stop cron server: %v", err)
	}

	err := <-startCh
	assert.NoError(t, err, "cron server should start without error")
}
