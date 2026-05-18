package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
)

func newTestBase(parentCtx context.Context) app.Base {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Release = "abcdefg1234567"

	return app.NewBase(
		parentCtx,
		mocks.NewMockedAuthService("test-user"),
		logging.NewNopLogger(),
		cfg,
	)
}

func TestNewBase_SetsAllFields(t *testing.T) {
	b := newTestBase(context.Background())
	defer b.CtxCancel()

	assert.NotNil(t, b.Logger)
	assert.NotNil(t, b.Ctx)
	assert.NotNil(t, b.CtxCancel)
	assert.NotNil(t, b.Auth)
}

func TestNewBase_WithParentContextCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	b := newTestBase(parent)
	defer b.CtxCancel()

	require.NoError(t, b.Ctx.Err())

	cancel()

	<-b.Ctx.Done()
	assert.ErrorIs(t, b.Ctx.Err(), context.Canceled)
}

func TestBase_GetDomain(t *testing.T) {
	b := newTestBase(context.Background())
	defer b.CtxCancel()
	assert.Equal(t, "", b.GetDomain())
}

func TestBase_GetDisplayName(t *testing.T) {
	b := newTestBase(context.Background())
	defer b.CtxCancel()
	assert.Equal(t, "", b.GetDisplayName())
}
