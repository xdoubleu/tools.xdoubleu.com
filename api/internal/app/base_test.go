package app_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mocks"
)

func newTestBase(parentCtx context.Context) app.Base {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = config.TestEnv
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

func TestBase_ShutdownCancelsCtxAndDrainsBackground(t *testing.T) {
	b := newTestBase(context.Background())

	release := make(chan struct{})
	var finished atomic.Bool
	b.Background.Go(t.Context(), "slow", func(context.Context) {
		<-release
		finished.Store(true)
	})

	shutdown := make(chan error, 1)
	go func() { shutdown <- b.Shutdown(t.Context()) }()

	<-b.Ctx.Done()
	close(release)
	require.NoError(t, <-shutdown)
	assert.True(t, finished.Load())
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
