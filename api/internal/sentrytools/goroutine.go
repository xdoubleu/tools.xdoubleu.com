package sentrytools

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getsentry/sentry-go"
)

// GoRoutineWrapper wraps a go routine with Sentry logic for error and
// performance tracking, starting a transaction that spans f's entire
// execution. Use this for a goroutine whose lifetime is one bounded unit of
// work -- a goroutine that instead loops indefinitely (e.g. a worker pool's
// run loop) should use SetupGoRoutineHub, so each unit of work dispatched
// from inside the loop gets its own transaction rather than all of them
// nesting under one that never finishes.
func GoRoutineWrapper(
	ctx context.Context,
	logger *slog.Logger,
	name string,
	f func(ctx context.Context, logger *slog.Logger) error,
) {
	name = fmt.Sprintf("GO ROUTINE %s", name)

	hub := sentry.CurrentHub().Clone()
	ctx = sentry.SetHubOnContext(ctx, hub)

	options := []sentry.SpanOption{
		sentry.WithOpName("go.routine"),
	}

	transaction := sentry.StartTransaction(ctx, name, options...)
	transaction.Status = sentry.HTTPtoSpanStatus(http.StatusOK)
	defer transaction.Finish()

	err := f(transaction.Context(), logger)
	if err != nil {
		transaction.Status = sentry.HTTPtoSpanStatus(http.StatusInternalServerError)
	}

	captureError(hub, err)
}

// SetupGoRoutineHub clones the current Sentry hub onto ctx -- which is what
// lets slog.Error calls inside f reach Sentry via sentrytools.NewLogHandler
// -- and reports any error f returns, without starting a Sentry transaction
// of its own. Use this for a goroutine that runs indefinitely; the work it
// dispatches should start its own per-run transaction instead of inheriting
// one that spans the goroutine's whole lifetime.
func SetupGoRoutineHub(
	ctx context.Context,
	logger *slog.Logger,
	f func(ctx context.Context, logger *slog.Logger) error,
) {
	hub := sentry.CurrentHub().Clone()
	ctx = sentry.SetHubOnContext(ctx, hub)

	err := f(ctx, logger)

	captureError(hub, err)
}

func captureError(hub *sentry.Hub, err error) {
	if err == nil {
		return
	}

	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		hub.CaptureException(err)
	})
}
