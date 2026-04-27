package app_test

import (
	"context"
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

//go:embed templates/html
var testAppTemplates embed.FS

func newTestBase(parentCtx context.Context) app.Base {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Release = "abcdefg1234567"

	return app.NewBase(
		parentCtx,
		mocks.NewMockedAuthService("test-user"),
		logging.NewNopLogger(),
		cfg,
		testAppTemplates,
		templates.LoadShared(cfg),
	)
}

func TestNewBase_SetsAllFields(t *testing.T) {
	b := newTestBase(context.Background())
	defer b.CtxCancel()

	assert.NotNil(t, b.Logger)
	assert.NotNil(t, b.Ctx)
	assert.NotNil(t, b.CtxCancel)
	assert.NotNil(t, b.Tpl)
	assert.NotNil(t, b.Auth)
}

func TestNewBase_TemplateCloned(t *testing.T) {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Release = "abcdefg1234567"
	shared := templates.LoadShared(cfg)

	b := app.NewBase(
		context.Background(),
		mocks.NewMockedAuthService("test-user"),
		logging.NewNopLogger(),
		cfg,
		testAppTemplates,
		shared,
	)
	defer b.CtxCancel()

	assert.NotSame(t, shared, b.Tpl)
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
