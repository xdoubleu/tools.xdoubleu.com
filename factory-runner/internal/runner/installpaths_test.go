package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestInstalledBundleFallsBackToSupportWhenApplicationsIsUnwritable(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(runner.ApplicationsDir(home)), 0o755))
	require.NoError(t, os.WriteFile(runner.ApplicationsDir(home), nil, 0o600))
	assert.Equal(t, runner.SupportBundlePath(home), runner.InstalledBundlePath(home))
	assert.Equal(t, filepath.Join(runner.SupportBundlePath(home),
		"Contents", "MacOS", "factory-runner"), runner.SupportBinaryPath(home))
}

func TestLaunchAgentRunsReadsThePlist(t *testing.T) {
	home := t.TempDir()
	assert.False(t, runner.LaunchAgentRuns(home, "/a/b"))
	_, err := runner.InstallLaunchAgent(home, "/a/b")
	require.NoError(t, err)
	assert.True(t, runner.LaunchAgentRuns(home, "/a/b"))
	assert.False(t, runner.LaunchAgentRuns(home, "/a/c"))
}

func TestRemoveSupportBundle(t *testing.T) {
	home := t.TempDir()
	bin := runner.SupportBinaryPath(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0o755))
	require.NoError(t, runner.RemoveSupportBundle(home))
	assert.NoDirExists(t, runner.SupportBundlePath(home))

	// A file where the install dir should be makes the removal fail.
	blocked := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(runner.InstallDir(blocked)), 0o755))
	require.NoError(t, os.WriteFile(runner.InstallDir(blocked), nil, 0o600))
	assert.Error(t, runner.RemoveSupportBundle(blocked))
}
