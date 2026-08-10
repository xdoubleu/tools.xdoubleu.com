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
