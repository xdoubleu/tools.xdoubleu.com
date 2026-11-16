package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// A process with no menu bar and no control sets mode, repos and pairing through
// the CLI, and the menu bar's actions see the same config.
func TestTheCLIDrivesTheRunnerHeadlessAndTheMenuSeesIt(t *testing.T) {
	e := newEnv(t)
	e.app.menu, e.app.control = nil, nil

	require.Equal(t, 0, e.do("pair", "code123"))
	require.Equal(t, 0, e.do("config", "mode", "claude"))
	require.Equal(t, 0, e.do("config", "repos", "add", "xdoubleu/tools"))

	assert.Equal(t, runner.ModeClaude, e.app.configMode())
	assert.Equal(t, []string{"xdoubleu/tools"}, e.app.repos())

	require.NoError(t, e.app.removeRepo("xdoubleu/tools"))
	require.NoError(t, e.app.chooseMode(runner.ModeOff))
	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "mode: off")
	assert.Empty(t, e.app.repos())
}

func TestInstallPutsTheCommandOnPath(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("b"), 0o755))
	e.app.execPath = src
	require.Equal(t, 0, e.do("install"))

	link := filepath.Join(e.app.home, "bin", "factory-runner")
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, runner.InstalledBinaryPath(e.app.home), target)
	assert.Contains(t, e.out.String(), link)
}

func TestInstallCarriesOnWhenTheCommandCannotBeLinked(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("b"), 0o755))
	e.app.execPath = src
	require.NoError(t, os.WriteFile(filepath.Join(e.app.home, "bin"), nil, 0o600))

	require.Equal(t, 0, e.do("install"))
	assert.Contains(t, e.errs.String(), "link the factory-runner command")
}
