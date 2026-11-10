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
	plist := runner.LaunchAgentPlistForTest(
		"/Users/a&b", "/Users/a&b/<bin>/factory-runner",
	)
	assert.Contains(t, plist, "<string>/Users/a&amp;b/&lt;bin&gt;/factory-runner</string>")
	assert.Contains(t, plist, "<string>run</string>")
	assert.Contains(t, plist, "<key>StandardErrorPath</key>")
	assert.Contains(t, plist, "/Users/a&amp;b/Library/Logs/factory-runner.log")
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
	assert.FileExists(t,
		filepath.Join(runner.InstalledBundlePath(home), "Contents", "Info.plist"))

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

	// An app that isn't there can't be copied, and leaves nothing behind.
	other := t.TempDir()
	gone := filepath.Join(t.TempDir(), "Gone.app", "Contents", "MacOS", "factory-runner")
	_, err = runner.InstallBinary(other, gone)
	require.Error(t, err)
	assert.NoDirExists(t, runner.InstalledBundlePath(other)+".install")
}

func TestLoginItemFollowsThePlist(t *testing.T) {
	home := t.TempDir()
	assert.False(t, runner.LoginItemEnabled(home))

	_, err := runner.InstallLaunchAgent(home, "/usr/local/bin/factory-runner")
	require.NoError(t, err)
	assert.True(t, runner.LoginItemEnabled(home))

	require.NoError(t, runner.DisableLoginItem(home))
	assert.False(t, runner.LoginItemEnabled(home))
}

func TestDisableLoginItemWhenNeverEnabled(t *testing.T) {
	require.NoError(t, runner.DisableLoginItem(t.TempDir()))
}

func TestBinaryReleaseReadsTheVersionOutput(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho abc123\n"), 0o755))
	assert.Equal(t, "abc123", runner.BinaryRelease(bin))
	assert.Empty(t, runner.BinaryRelease(filepath.Join(t.TempDir(), "missing")))
}
