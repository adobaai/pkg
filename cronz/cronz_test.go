package cronz

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	log := slog.Default()
	s := New(log)

	// Register a job
	count := 0
	if err := s.Register("test-job", "*/1 * * * *", func(ctx context.Context) error {
		log.Info("test job running")
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
	time.Sleep(2 * time.Minute)
	log.Info("test job ran successfully")
	assert.Equal(t, 2, count, "job should run twice")

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("failed to stop cron server: %v", err)
	}

	err := <-startCh
	assert.NoError(t, err, "cron server should start without error")
}
