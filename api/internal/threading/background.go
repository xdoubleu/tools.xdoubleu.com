package threading

import (
	"context"
	"fmt"
	"log/slog"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/sentrytools"
)

// Background runs work a request starts but doesn't wait for. A task's panic
// is logged at Error under its name, which reaches Sentry; with a limit, at
// most that many tasks run at once and the rest wait their turn. Shutdown
// drains the tasks.
type Background struct {
	logger   *slog.Logger
	ctx      context.Context // cancelled once Shutdown gives up
	cancel   context.CancelFunc
	slots    chan struct{} // nil when unbounded
	inflight *inflight
}

// NewBackground creates a [Background]; limit <= 0 means unbounded.
func NewBackground(logger *slog.Logger, limit int) *Background {
	ctx, cancel := context.WithCancel(context.Background())
	var slots chan struct{}
	if limit > 0 {
		slots = make(chan struct{}, limit)
	}
	return &Background{
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		slots:    slots,
		inflight: newInflight(),
	}
}

// Go runs fn on its own goroutine. fn's context keeps ctx's values but not
// its cancellation or deadline, so the task outlives the request. Once
// Shutdown gives up, running tasks are cancelled and waiting ones never run.
func (b *Background) Go(ctx context.Context, name string, fn func(context.Context)) {
	b.inflight.add()
	taskCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(b.ctx, cancel)

	go func() {
		defer b.inflight.done()
		defer cancel()
		defer stop()

		if b.slots != nil {
			select {
			case b.slots <- struct{}{}:
				defer func() { <-b.slots }()
			case <-taskCtx.Done():
			}
		}
		if b.ctx.Err() != nil {
			return // Shutdown gave up before the task started
		}

		sentrytools.SetupGoRoutineHub(taskCtx, b.logger, b.run(name, fn))
	}()
}

func (b *Background) run(
	name string,
	fn func(context.Context),
) func(context.Context, *slog.Logger) error {
	return func(ctx context.Context, logger *slog.Logger) error {
		defer Recover(ctx, logger, name)
		fn(ctx)
		return nil
	}
}

// Recover, deferred directly, turns a panic into an Error log under name
// (which reaches Sentry through ctx's hub) so the process survives it.
func Recover(ctx context.Context, logger *slog.Logger, name string) {
	if r := recover(); r != nil {
		logger.ErrorContext(ctx, "goroutine panicked",
			slog.String("task", name),
			logging.ErrAttr(fmt.Errorf("panic: %v", r)))
	}
}

// Shutdown waits for every task to finish. Once ctx is done it cancels the
// tasks still running and returns ctx's error without waiting for them.
func (b *Background) Shutdown(ctx context.Context) error {
	err := b.inflight.wait(ctx)
	if err != nil {
		b.cancel()
	}
	return err
}
