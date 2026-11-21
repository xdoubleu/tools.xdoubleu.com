package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func resourcePath(bundle string) string {
	return filepath.Join(bundle, "Contents", "Resources", "AppIcon.icns")
}

func TestInstallBinaryFromBareBinaryWritesIconAndVersion(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("bin"), 0o755))
	_, err := runner.InstallBinary(home, src, "v9")
	require.NoError(t, err)

	bundle := runner.InstalledBundlePath(home)
	icns, err := os.ReadFile(resourcePath(bundle))
	require.NoError(t, err)
	assert.Equal(t, "icns", string(icns[:4]))
	plist, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	require.NoError(t, err)
	assert.Contains(t, string(plist), "<key>CFBundleIconFile</key>")
	assert.Contains(t, string(plist), "<string>v9</string>")
	assert.NotContains(t, string(plist), "__RELEASE__")
}

func TestRepairBundleFillsAMinimalBundle(t *testing.T) {
	home := t.TempDir()
	repaired, err := runner.RepairBundle(home, "v1")
	require.NoError(t, err)
	assert.False(t, repaired, "nothing installed")

	bin := runner.InstalledBinaryPath(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0o755))
	require.NoError(t, os.WriteFile(bin, []byte("bin"), 0o755))
	repaired, err = runner.RepairBundle(home, "v1")
	require.NoError(t, err)
	assert.True(t, repaired)
	assert.FileExists(t, resourcePath(runner.InstalledBundlePath(home)))
	got, _ := os.ReadFile(bin)
	assert.Equal(t, "bin", string(got))

	repaired, err = runner.RepairBundle(home, "v1")
	require.NoError(t, err)
	assert.False(t, repaired, "already complete")
}

func TestRepairBundleReportsAnUnwritableBundle(t *testing.T) {
	home := t.TempDir()
	bin := runner.InstalledBinaryPath(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0o755))
	require.NoError(t, os.WriteFile(bin, []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(runner.InstalledBundlePath(home), "Contents", "Resources"), nil, 0o644))
	_, err := runner.RepairBundle(home, "v1")
	assert.Error(t, err)
}
