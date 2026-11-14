package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func menuEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	e.app.control = runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	e.app.control.SetModeSource(e.app.mode)
	return e
}

func TestMenuModeWritesTheConfigAndTheStopFlag(t *testing.T) {
	e := menuEnv(t)
	assert.Equal(t, runner.ModeOff, e.app.configMode())

	require.NoError(t, e.app.chooseMode(runner.ModeClaude))
	assert.Equal(t, runner.ModeClaude, e.app.configMode())
	assert.False(t, e.app.control.Paused())

	require.NoError(t, e.app.chooseMode(runner.ModeOff))
	assert.Equal(t, runner.ModeOff, e.app.configMode())
	assert.True(t, e.app.control.Paused())

	require.Error(t, e.app.chooseMode("opencode"))
}

func TestMenuReposAddAndRemove(t *testing.T) {
	e := menuEnv(t)
	require.NoError(t, e.app.addRepo("xdoubleu/tools"))
	require.NoError(t, e.app.addRepo("xdoubleu/api"))
	assert.Equal(t, []string{"xdoubleu/api", "xdoubleu/tools"}, e.app.repos())

	require.Error(t, e.app.addRepo("not a repo"))
	require.NoError(t, e.app.removeRepo("xdoubleu/api"))
	assert.Equal(t, []string{"xdoubleu/tools"}, e.app.repos())
	require.Error(t, e.app.removeRepo("xdoubleu/api"))
}

func TestPairWithKeepsAFreshMacOffAndTakesTheServer(t *testing.T) {
	e := menuEnv(t)
	e.app.control.SetUnpaired()
	e.store.token = ""

	_, err := e.app.pairWith(context.Background(), "ABC", "https://example.com/api")
	require.NoError(t, err)
	assert.Equal(t, "frt_ABC", e.store.token)
	assert.Equal(t, []string{"https://example.com/api"}, e.server.servers)
	assert.Equal(t, "https://example.com/api", e.app.serverURL())
	assert.Equal(t, runner.ModeOff, e.app.configMode())
	assert.False(t, e.app.control.Snapshot().Unpaired)

	_, err = e.app.pairWith(context.Background(), "ABC", "http://example.com")
	require.Error(t, err, "plain http is refused")
}

func TestPairWithKeepsAChosenMode(t *testing.T) {
	e := menuEnv(t)
	require.NoError(t, e.app.chooseMode(runner.ModeClaude))
	_, err := e.app.pairWith(context.Background(), "ABC", "")
	require.NoError(t, err)
	assert.Equal(t, runner.ModeClaude, e.app.configMode())
}

func TestAnUnpairedRunnerWaitsInTheMenuForAPairing(t *testing.T) {
	e := menuEnv(t)
	e.store.token = ""
	e.store.getErr = runner.ErrNoToken
	e.app.menu = func(
		ctx context.Context, info menuInfo, work func(context.Context) error,
	) error {
		assert.True(t, info.Control.Snapshot().Unpaired)
		assert.Equal(t, "Not paired", info.Control.Snapshot().Line())
		go func() {
			e.store.getErr = nil
			err := info.Pair(ctx, "XYZ", "")
			assert.NoError(t, err)
		}()
		return work(ctx)
	}
	assert.Equal(t, 0, e.do("run"))
	assert.Equal(t, []string{"frt_XYZ"}, *e.ran)
	assert.Equal(t, runner.ModeOff, e.app.configMode(), "a fresh pairing stays off")
}

func TestAnUnpairedRunnerStopsWaitingWhenTheMenuQuits(t *testing.T) {
	e := menuEnv(t)
	e.store.getErr = runner.ErrNoToken
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.app.awaitPairing(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
