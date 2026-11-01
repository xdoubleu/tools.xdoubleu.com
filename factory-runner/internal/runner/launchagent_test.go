package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestPlistEscapesThePathAndRunsTheDaemon(t *testing.T) {
	plist := runner.LaunchAgentPlistForTest("/Users/a&b/<bin>/factory-runner")
	assert.Contains(t, plist, "<string>/Users/a&amp;b/&lt;bin&gt;/factory-runner</string>")
	assert.Contains(t, plist, "<string>run</string>")
	assert.Contains(t, plist, runner.LaunchAgentLabel)
}

func TestInstallWritesThePlistUnderLaunchAgents(t *testing.T) {
	home := t.TempDir()
	path, err := runner.InstallLaunchAgent(home, "/usr/local/bin/factory-runner")
	require.NoError(t, err)
	assert.Equal(t, runner.LaunchAgentPath(home), path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "/usr/local/bin/factory-runner")
}

func TestInstallFailsWhereLaunchAgentsCannotBeCreated(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "Library"), nil, 0o600))
	_, err := runner.InstallLaunchAgent(home, "x")
	require.Error(t, err)

	home = t.TempDir()
	require.NoError(t, os.MkdirAll(runner.LaunchAgentPath(home), 0o755))
	_, err = runner.InstallLaunchAgent(home, "x")
	require.Error(t, err)
}

func TestInstallBinaryCopiesTheExecutableToAStablePath(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("binary"), 0o755))

	dst, err := runner.InstallBinary(home, src)
	require.NoError(t, err)
	assert.Equal(t, runner.InstalledBinaryPath(home), dst)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "binary", string(data))
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	// Installing again, from the installed copy, changes nothing.
	again, err := runner.InstallBinary(home, dst)
	require.NoError(t, err)
	assert.Equal(t, dst, again)
}

func TestInstallBinaryFailures(t *testing.T) {
	home := t.TempDir()
	_, err := runner.InstallBinary(home, filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err, "no source")

	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("b"), 0o755))
	blocked := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "Library"), nil, 0o600))
	_, err = runner.InstallBinary(blocked, src)
	require.Error(t, err, "install dir can't be created")

	// A directory in the way makes the final rename fail and cleans up.
	other := t.TempDir()
	require.NoError(t, os.MkdirAll(
		filepath.Join(runner.InstalledBinaryPath(other), "child"), 0o755))
	_, err = runner.InstallBinary(other, src)
	require.Error(t, err)
	assert.NoFileExists(t, runner.InstalledBinaryPath(other)+".install")
}
