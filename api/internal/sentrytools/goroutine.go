package sentrytools

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getsentry/sentry-go"
)

// GoRoutineWrapper runs f in one Sentry transaction. For a goroutine that
// loops forever, use SetupGoRoutineHub so each unit of work gets its own.
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

// SetupGoRoutineHub clones the Sentry hub onto ctx (so slog.Error reaches
// Sentry) and reports f's error, without starting a transaction.
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
