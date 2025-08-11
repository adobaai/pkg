package cronz

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/rs/xid"

	"github.com/adobaai/pkg/middleware"
)

type Context interface {
	context.Context
	Name() string
	Spec() string
	RunID() string
	WithContext(context.Context) Context
}

type jobContext struct {
	context.Context
	name  string
	spec  string
	runID string
}

func newContext(ctx context.Context, name, spec string) Context {
	return &jobContext{
		Context: ctx,
		name:    name,
		spec:    spec,
		runID:   xid.New().String(),
	}
}

func (c *jobContext) Name() string {
	return c.name
}

func (c *jobContext) Spec() string {
	return c.spec
}

func (c *jobContext) RunID() string {
	return c.runID
}

// WithContext replaces current context with a new one.
func (c *jobContext) WithContext(ctx context.Context) Context {
	return &jobContext{
		Context: ctx,
		name:    c.name,
		spec:    c.spec,
		runID:   c.runID,
	}
}

type Action = middleware.Handler[Context]

// Server is a cron server which implements kraots [transport.Server].
type Server struct {
	cron   *cron.Cron
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	mws    []Middleware
	copts  []cron.Option

	nameToID sync.Map
}

// Option is the new server option.
type Option func(s *Server)

// WithMiddlewares adds middleware to the cron server.
func WithMiddlewares(mws ...Middleware) Option {
	return func(s *Server) {
		s.mws = append(s.mws, mws...)
	}
}

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option {
	return func(s *Server) {
		s.logger = log.With("component", "cronz")
	}
}

// WithCronOptions sets the [cron] options.
func WithCronOptions(opts ...cron.Option) Option {
	return func(s *Server) {
		s.copts = append(s.copts, opts...)
	}
}

// New creates a new instance of the cron server.
func New(opts ...Option) *Server {
	s := &Server{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.cron = cron.New(s.copts...)
	return s
}

// Use adds middleware to the cron server
func (s *Server) Use(mws ...Middleware) {
	s.mws = append(s.mws, mws...)
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

// Register registers a job by specifying its name, spec and action.
func (s *Server) Register(
	name string,
	spec string,
	action Action,
) error {
	// At the register time, the server context is not available yet.
	ctx := newContext(context.Background(), name, spec)
	chain := middleware.Chain(s.mws...)(action)
	id, err := s.cron.AddJob(spec, s.cronJob(ctx, chain))
	if err != nil {
		return err
	}

	s.nameToID.Store(name, id)
	s.logger.Info("registered cron job", "name", name, "spec", spec)
	return nil
}

func (s *Server) cronJob(ctx Context, action Action) cron.Job {
	return cron.FuncJob(func() {
		ctx := ctx.WithContext(s.ctx)
		select {
		default:
			_ = action(ctx)
		case <-ctx.Done():
			return
		}
	})
}
