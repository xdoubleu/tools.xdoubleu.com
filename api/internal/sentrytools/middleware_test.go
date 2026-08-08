package sentrytools_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/sentrytools"
)

func TestInitSkippedForTestEnv(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //other fields are optional
	hub, err := sentrytools.Init(
		config.TestEnv,
		sentry.ClientOptions{Dsn: "http://x@example.com/1"},
	)
	require.NoError(t, err)
	assert.Nil(t, hub)
}

func TestInitSkippedForEmptyDsn(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //other fields are optional
	hub, err := sentrytools.Init(config.ProdEnv, sentry.ClientOptions{Dsn: ""})
	require.NoError(t, err)
	assert.Nil(t, hub)
}

func TestMiddlewareTestEnv(t *testing.T) {
	t.Parallel()

	mw, err := sentrytools.Middleware(config.TestEnv)
	require.NoError(t, err)
	assert.NotNil(t, mw)
}
