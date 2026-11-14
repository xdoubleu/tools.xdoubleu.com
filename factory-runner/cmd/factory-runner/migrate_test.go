package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func seedSupportApp(t *testing.T, e env) {
	t.Helper()
	bin := runner.SupportBinaryPath(e.app.home)
	require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0o755))
	require.NoError(t, os.WriteFile(bin, []byte("b"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(runner.InstallDir(e.app.home), "config.json"), []byte("{}"), 0o600))
}

func TestRunMovesTheSupportCopyIntoApplications(t *testing.T) {
	e := newEnv(t)
	seedSupportApp(t, e)
	e.app.execPath = runner.SupportBinaryPath(e.app.home)

	assert.Equal(t, 0, e.do("run"))
	assert.Empty(t, *e.ran, "the daemon waits for the reload")
	apps := filepath.Join(runner.ApplicationsDir(e.app.home), "FactoryRunner.app")
	assert.FileExists(t, filepath.Join(apps, "Contents", "MacOS", "factory-runner"))
	bin := runner.InstalledBinaryPath(e.app.home)
	assert.True(t, runner.LaunchAgentRuns(e.app.home, bin))

	// Restarted from /Applications: the old copy goes, config stays.
	e.app.execPath = runner.InstalledBinaryPath(e.app.home)
	e.do("run")
	assert.NoDirExists(t, runner.SupportBundlePath(e.app.home))
	assert.FileExists(t, filepath.Join(runner.InstallDir(e.app.home), "config.json"))
}

func TestRunKeepsTheSupportCopyUntilTheLaunchAgentMoved(t *testing.T) {
	e := newEnv(t)
	seedSupportApp(t, e)
	require.NoError(t, os.MkdirAll(
		filepath.Dir(runner.InstalledBinaryPath(e.app.home)), 0o755))
	require.NoError(t, os.WriteFile(runner.InstalledBinaryPath(e.app.home), nil, 0o755))
	_, err := runner.InstallLaunchAgent(e.app.home, runner.SupportBinaryPath(e.app.home))
	require.NoError(t, err)
	e.app.execPath = runner.InstalledBinaryPath(e.app.home)

	e.do("run")
	assert.DirExists(t, runner.SupportBundlePath(e.app.home))
}

func TestRunStaysPutWhenApplicationsIsNotWritable(t *testing.T) {
	e := newEnv(t)
	seedSupportApp(t, e)
	require.NoError(t, os.MkdirAll(filepath.Join(e.app.home, "Library"), 0o755))
	// A file where the Applications folder should be makes it unwritable.
	require.NoError(t, os.WriteFile(runner.ApplicationsDir(e.app.home), nil, 0o600))
	e.app.execPath = runner.SupportBinaryPath(e.app.home)

	assert.Equal(t, runner.SupportBinaryPath(e.app.home),
		runner.InstalledBinaryPath(e.app.home))
	e.do("run")
	assert.Contains(t, e.errs.String(), "not writable")
	assert.Equal(t, []string{"frt_stored"}, *e.ran, "keeps running from where it is")
	assert.DirExists(t, runner.SupportBundlePath(e.app.home))
}
