package sentrytools_test

import (
	"context"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/sentrytools"
)

func TestMockedSentryHubCapturesEvents(t *testing.T) {
	t.Parallel()

	hub := sentrytools.MockedSentryHub()
	require.NotNil(t, hub)

	hub.CaptureMessage("test event")
	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)

	transport := hub.Client().Transport
	assert.True(t, transport.Flush(time.Second))
	assert.True(t, transport.FlushWithContext(context.Background()))
	transport.Configure(sentrytools.MockedSentryClientOptions())
	transport.Close()
}

func TestMockedHubEventsPanicsForWrongHub(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //other fields are optional
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: ""})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())

	assert.Panics(t, func() {
		sentrytools.MockedHubEvents(hub)
	})
}
