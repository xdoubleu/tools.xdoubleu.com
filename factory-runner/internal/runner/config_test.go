package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestMissingConfigIsTheDefault(t *testing.T) {
	cfg, err := runner.LoadConfig(filepath.Join(t.TempDir(), "none.json"))
	require.NoError(t, err)
	assert.Equal(t, runner.DefaultServer, cfg.Server)
	assert.Empty(t, cfg.Repos)
}

func TestConfigRoundTripsOwnerOnly(t *testing.T) {
	path := runner.ConfigPath(t.TempDir())
	cfg, err := runner.LoadConfig(path)
	require.NoError(t, err)
	require.NoError(t, cfg.AddRepo("b/two"))
	require.NoError(t, cfg.AddRepo("a/one"))
	require.NoError(t, cfg.AddRepo("A/ONE"), "case-insensitive duplicate")
	require.NoError(t, cfg.Save(path))

	got, err := runner.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"a/one", "b/two"}, got.Repos)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAConfigWithoutAServerGetsTheDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"repos":["a/b"]}`), 0o600))
	cfg, err := runner.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, runner.DefaultServer, cfg.Server)
}

func TestBrokenOrUnreadableConfigIsAnError(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("{"), 0o600))
	_, err := runner.LoadConfig(bad)
	require.Error(t, err)

	_, err = runner.LoadConfig(dir) // a directory can't be read as a file
	require.Error(t, err)

	cfg := runner.Config{Server: "x", Repos: nil}
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	require.Error(t, cfg.Save(filepath.Join(blocker, "sub", "c.json")))
}

func TestAddAndRemoveRepos(t *testing.T) {
	cfg := runner.Config{Server: "x", Repos: []string{}}
	require.Error(t, cfg.AddRepo("not-a-repo"))
	require.Error(t, cfg.AddRepo("-a/b"))
	require.NoError(t, cfg.AddRepo("a/b"))
	assert.False(t, cfg.RemoveRepo("a/zzz"))
	assert.True(t, cfg.RemoveRepo("A/B"))
	assert.Empty(t, cfg.Repos)
}

func TestServerMustBeHTTPSOrLoopback(t *testing.T) {
	cfg := runner.Config{Server: "x", Repos: nil}
	for _, ok := range []string{
		"https://example.com/api/", "http://localhost:8080", "http://127.0.0.1",
		"http://localhost/api",
	} {
		require.NoError(t, cfg.SetServer(ok), ok)
	}
	assert.Equal(t, "http://localhost/api", cfg.Server)
	for _, bad := range []string{
		"", "https://", "http://example.com", "http://localhost.evil.com",
		"http://localhostevil", "ftp://x", "example.com",
	} {
		require.Error(t, cfg.SetServer(bad), bad)
	}
}
