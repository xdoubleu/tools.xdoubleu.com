package sentrytools_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/sentrytools"
)

func TestMiddlewareTestEnv(t *testing.T) {
	t.Parallel()

	mw, err := sentrytools.Middleware()
	require.NoError(t, err)
	assert.NotNil(t, mw)
}
