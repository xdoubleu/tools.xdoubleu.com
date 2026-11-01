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
