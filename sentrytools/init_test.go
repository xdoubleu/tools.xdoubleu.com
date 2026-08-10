package sentrytools_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/sentrytools"
)

func TestInit_SkippedForTestEnv(t *testing.T) {
	t.Parallel()

	hub, err := sentrytools.Init(
		"test",
		sentry.ClientOptions{Dsn: "http://x@example.com/1"},
	)
	require.NoError(t, err)
	assert.Nil(t, hub)
}

func TestInit_SkippedForEmptyDsn(t *testing.T) {
	t.Parallel()

	hub, err := sentrytools.Init("production", sentry.ClientOptions{Dsn: ""})
	require.NoError(t, err)
	assert.Nil(t, hub)
}

func TestInit_Success(t *testing.T) {
	// Not t.Parallel(): mutates the process-global Sentry hub via the real
	// sentry.Init, unlike the other tests here (which skip before touching
	// it) and the loghandler tests (which build their own local hubs).
	hub, err := sentrytools.Init(
		"production",
		sentry.ClientOptions{Dsn: "http://whatever@example.com/1337"},
	)
	require.NoError(t, err)
	assert.NotNil(t, hub)
}
