package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Kamal SIGKILLs a container kamalStopWait (its default stop_wait_time) after
// SIGTERM; these plus logFlushTimeout must stay under it.
const (
	kamalStopWait       = 30 * time.Second
	httpShutdownTimeout = 8 * time.Second
	drainTimeout        = 10 * time.Second
	// usageFlushTimeout is kept apart so a slow drain can't cost the counts.
	usageFlushTimeout  = 3 * time.Second
	sentryFlushTimeout = 2 * time.Second
)

// newRootContext is the process's root context, cancelled on SIGINT or
// SIGTERM.
func newRootContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// Shutdown drains what outlives the HTTP server: each app's background tasks
// and jobs, the cross-app jobs and queued email, then the usage counts. Call
// it after cancelling the root context.
func (app *Application) Shutdown(ctx context.Context) error {
	drainCtx, cancel := context.WithTimeout(ctx, drainTimeout)
	defer cancel()

	appsErr := app.apps.Shutdown(drainCtx)
	jobsErr := app.globalJobQueue.Wait(drainCtx)
	mailErr := app.notifications.Shutdown(drainCtx)

	flushCtx, cancelFlush := context.WithTimeout(ctx, usageFlushTimeout)
	defer cancelFlush()

	return errors.Join(appsErr, jobsErr, mailErr, app.usage.Stop(flushCtx))
}
