package middleware_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/middleware"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	handlers, err := middleware.Default(
		logging.NewNopLogger(),
		[]string{"http://localhost"},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, handlers)
}
