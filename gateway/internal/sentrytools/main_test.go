package sentrytools_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/config"
	"tools.xdoubleu.com/gateway/internal/sentrytools"
)

func TestInitSkippedForTestEnv(t *testing.T) {
	t.Parallel()

	hub, err := sentrytools.Init(
		config.TestEnv,
		sentry.ClientOptions{Dsn: "http://x@example.com/1"},
	)
	require.NoError(t, err)
	assert.Nil(t, hub)
}

func TestInitSkippedForEmptyDsn(t *testing.T) {
	t.Parallel()

	hub, err := sentrytools.Init(
		config.ProdEnv,
		sentry.ClientOptions{Dsn: ""},
	)
	require.NoError(t, err)
	assert.Nil(t, hub)
}
