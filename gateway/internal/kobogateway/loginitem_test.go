package kobogateway_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

func TestLoginItemPath(t *testing.T) {
	path := kobogateway.LoginItemPath("/Users/test")

	assert.Equal(
		t,
		"/Users/test/Library/LaunchAgents/com.xdoubleu.tools.kobo-gateway.plist",
		path,
	)
}

func TestEnableLoginItemWritesPlist(t *testing.T) {
	home := t.TempDir()

	require.NoError(t, kobogateway.EnableLoginItem(home, "/usr/local/bin/kobo-gateway"))

	assert.True(t, kobogateway.LoginItemEnabled(home))

	raw, err := os.ReadFile(kobogateway.LoginItemPath(home))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "/usr/local/bin/kobo-gateway")
	assert.Contains(t, string(raw), "com.xdoubleu.tools.kobo-gateway")
	assert.Contains(t, string(raw), "RunAtLoad")
}

func TestDisableLoginItemRemovesPlist(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, kobogateway.EnableLoginItem(home, "/usr/local/bin/kobo-gateway"))

	require.NoError(t, kobogateway.DisableLoginItem(home))

	assert.False(t, kobogateway.LoginItemEnabled(home))
}

func TestDisableLoginItemWhenNeverEnabled(t *testing.T) {
	home := t.TempDir()

	assert.NoError(t, kobogateway.DisableLoginItem(home))
}

func TestLoginItemEnabledFalseInitially(t *testing.T) {
	home := t.TempDir()

	assert.False(t, kobogateway.LoginItemEnabled(home))
}

func TestEnsureInitialLoginItemRegistersOnce(t *testing.T) {
	home := t.TempDir()
	markerDir := t.TempDir()

	require.NoError(t, kobogateway.EnsureInitialLoginItem(
		markerDir, home, "/usr/local/bin/kobo-gateway",
	))
	assert.True(t, kobogateway.LoginItemEnabled(home))

	// User disables it; a second EnsureInitialLoginItem call (next launch)
	// must respect that and not re-enable it.
	require.NoError(t, kobogateway.DisableLoginItem(home))

	require.NoError(t, kobogateway.EnsureInitialLoginItem(
		markerDir, home, "/usr/local/bin/kobo-gateway",
	))
	assert.False(t, kobogateway.LoginItemEnabled(home))
}

func TestEnsureInitialLoginItemMarkerPersists(t *testing.T) {
	home := t.TempDir()
	markerDir := t.TempDir()

	require.NoError(t, kobogateway.EnsureInitialLoginItem(
		markerDir, home, "/usr/local/bin/kobo-gateway",
	))

	_, err := os.Stat(filepath.Join(markerDir, ".login-item-initialized"))
	assert.NoError(t, err)
}
