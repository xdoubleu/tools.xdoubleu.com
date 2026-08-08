package contexttools_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/contexttools"
)

func TestShowErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.False(t, contexttools.ShowErrors(ctx))

	ctx = contexttools.WithShownErrors(ctx)
	assert.True(t, contexttools.ShowErrors(ctx))
}

func TestLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// no logger set: falls back to a NopLogger instead of nil/panic.
	assert.NotNil(t, contexttools.Logger(ctx))
}
