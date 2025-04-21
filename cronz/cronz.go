package cronz

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

// Server is an implementation of the CronRegistrar interface
type Server struct {
	cron   *cron.Cron
	log    *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc

	nameToID sync.Map
}

// New creates a new instance of the cron server.
func New(log *slog.Logger) *Server {
	return &Server{
		cron: cron.New(),
		log:  log,
	}
}

// Start starts the cron scheduler.
// It will run all registered jobs according to their schedule.
// This method is a blocking call and will not return until the scheduler is stopped.
func (cs *Server) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	cs.cancel = cancel
	cs.ctx = ctx
	cs.cron.Run()
	return nil
}

// Stop stops the cron scheduler.
// It waits for all running jobs to finish before returning.
// If a job is running long, it will return an error.
// This is a blocking call.
//
// If you want to stop the cron server without waiting for jobs to finish,
// you can use a separate context with a timeout or cancellation.
// This will stop the cron server immediately without waiting for jobs to finish.
func (s *Server) Stop(ctx context.Context) error {
	stopCtx := s.cron.Stop()
	s.cancel()
	// OPTI What to do if a job is running long?
	select {
	case <-ctx.Done():
		return errors.New("cronz: still have running jobs")
	case <-stopCtx.Done():
		return nil
	}
}

// Register registers a task by specifying its name, spec and action.
func (s *Server) Register(
	name string,
	spec string,
	action func(context.Context) error,
) error {
	// OPTI: OpenTelemetry
	id, err := s.cron.AddJob(spec, cron.FuncJob(func() {
		l := slog.With("cron", name)
		ctx := s.ctx
		select {
		case <-ctx.Done():
			l.ErrorContext(ctx, "cron job cancelled", "name", name)
			return
		default:
		}

		l.InfoContext(ctx, "cron job started", "name", name)
		if err := action(ctx); err != nil {
			l.ErrorContext(ctx, "cron job failed", "name", name, "err", err)
		} else {
			l.InfoContext(ctx, "cron job succeeded", "name", name)
		}
	}))
	if err != nil {
		return err
	}

	s.nameToID.Store(name, id)
	s.log.Info("registered cron job", "name", name, "spec", spec)
	return nil
}
