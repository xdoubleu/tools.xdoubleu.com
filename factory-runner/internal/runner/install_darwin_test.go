package runner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/appbundle"
)

func verify(t *testing.T, app string) error {
	t.Helper()
	return exec.CommandContext(context.Background(),
		"codesign", "--verify", "--deep", "--strict", app).Run()
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(to, data, 0o755))
}

func TestInstalledAppKeepsAValidSignatureThroughInstallAndInPlaceUpdate(t *testing.T) {
	src := filepath.Join(t.TempDir(), "FactoryRunner.app")
	macOS := filepath.Join(src, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macOS, 0o755))
	binary := filepath.Join(macOS, "factory-runner")
	copyFile(t, "/usr/bin/true", binary)
	require.NoError(t, os.WriteFile(filepath.Join(src, "Contents", "Info.plist"),
		[]byte(runner.BundleInfoPlistForTest()), 0o644))
	ctx := context.Background()
	require.NoError(t, appbundle.Resign(ctx, binary))
	require.NoError(t, verify(t, src))

	home := t.TempDir()
	installed, err := runner.InstallBinary(home, binary)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(runner.ApplicationsDir(home), "FactoryRunner.app"),
		runner.InstalledBundlePath(home))
	require.NoError(t, appbundle.Resign(ctx, installed))
	require.NoError(t, verify(t, runner.InstalledBundlePath(home)))

	// A self-update swaps the binary in place; resealing makes it valid again.
	copyFile(t, "/bin/echo", installed)
	require.Error(t, verify(t, runner.InstalledBundlePath(home)))
	require.NoError(t, appbundle.Resign(ctx, installed))
	require.NoError(t, verify(t, runner.InstalledBundlePath(home)))
}
