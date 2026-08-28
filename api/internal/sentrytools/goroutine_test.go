package sentrytools_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/sentrytools"
)

// initSentryTestClient (re)points the global Sentry hub at a client with
// tracing enabled, so transactions actually get sampled and sent (see
// sentry.Span.sample), and returns its transport to read captured events
// back from. GoRoutineWrapper/SetupGoRoutineHub clone sentry.CurrentHub(),
// not any hub already on ctx, so exercising them means Init'ing the global
// hub rather than stashing one on context -- these tests must not run
// t.Parallel() with each other or with anything else that calls sentry.Init.
func initSentryTestClient(t *testing.T) *sentry.Hub {
	t.Helper()

	options := sentrytools.MockedSentryClientOptions()
	options.EnableTracing = true
	options.TracesSampleRate = 1.0

	require.NoError(t, sentry.Init(options))

	return sentry.CurrentHub()
}

func TestGoRoutineWrapper_FinishesTransactionAndCapturesError(t *testing.T) {
	hub := initSentryTestClient(t)

	sentrytools.GoRoutineWrapper(
		t.Context(),
		logging.NewNopLogger(),
		"test routine",
		func(_ context.Context, _ *slog.Logger) error { return errors.New("boom") },
	)

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 2)

	var sawTransaction, sawException bool
	for _, event := range events {
		if event.Type == "transaction" {
			sawTransaction = true
			assert.Equal(t, "GO ROUTINE test routine", event.Transaction)
			trace, ok := event.Contexts["trace"]
			require.True(t, ok)
			assert.Equal(t, sentry.SpanStatusInternalError, trace["status"])
		}
		if len(event.Exception) > 0 {
			sawException = true
			assert.Equal(t, "boom", event.Exception[0].Value)
		}
	}
	assert.True(t, sawTransaction, "expected a transaction event")
	assert.True(t, sawException, "expected an exception event")
}

func TestGoRoutineWrapper_NoErrorFinishesOKTransaction(t *testing.T) {
	hub := initSentryTestClient(t)

	sentrytools.GoRoutineWrapper(
		t.Context(),
		logging.NewNopLogger(),
		"test routine",
		func(_ context.Context, _ *slog.Logger) error { return nil },
	)

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)
	assert.Equal(t, "GO ROUTINE test routine", events[0].Transaction)
	trace, ok := events[0].Contexts["trace"]
	require.True(t, ok)
	assert.Equal(t, sentry.SpanStatusOK, trace["status"])
}

func TestSetupGoRoutineHub_CapturesReturnedErrorWithoutTransaction(t *testing.T) {
	hub := initSentryTestClient(t)

	sentrytools.SetupGoRoutineHub(
		t.Context(),
		logging.NewNopLogger(),
		func(_ context.Context, _ *slog.Logger) error { return errors.New("boom") },
	)

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)
	assert.Empty(t, events[0].Transaction)
	require.Len(t, events[0].Exception, 1)
	assert.Equal(t, "boom", events[0].Exception[0].Value)
}

func TestSetupGoRoutineHub_NoErrorCapturesNothing(t *testing.T) {
	hub := initSentryTestClient(t)

	sentrytools.SetupGoRoutineHub(
		t.Context(),
		logging.NewNopLogger(),
		func(_ context.Context, _ *slog.Logger) error { return nil },
	)

	events := sentrytools.MockedHubEvents(hub)
	assert.Empty(t, events)
}
