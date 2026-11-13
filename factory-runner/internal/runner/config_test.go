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

	cfg := runner.Config{
		Server:     "x",
		Repos:      nil,
		Image:      "",
		DataDir:    "",
		GoModCache: "",
		NPMCache:   "",
		BotEmail:   "",
		Mode:       "",
	}
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	require.Error(t, cfg.Save(filepath.Join(blocker, "sub", "c.json")))
}

func TestAddAndRemoveRepos(t *testing.T) {
	cfg := runner.Config{
		Server:     "x",
		Repos:      []string{},
		Image:      "",
		DataDir:    "",
		GoModCache: "",
		NPMCache:   "",
		BotEmail:   "",
		Mode:       "",
	}
	require.Error(t, cfg.AddRepo("not-a-repo"))
	require.Error(t, cfg.AddRepo("-a/b"))
	require.NoError(t, cfg.AddRepo("a/b"))
	assert.False(t, cfg.RemoveRepo("a/zzz"))
	assert.True(t, cfg.RemoveRepo("A/B"))
	assert.Empty(t, cfg.Repos)
}

func TestServerMustBeHTTPSOrLoopback(t *testing.T) {
	cfg := runner.Config{
		Server:     "x",
		Repos:      nil,
		Image:      "",
		DataDir:    "",
		GoModCache: "",
		NPMCache:   "",
		BotEmail:   "",
		Mode:       "",
	}
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

func TestConfigModeDefaultsToOffAndRejectsUnknownValues(t *testing.T) {
	var cfg runner.Config
	assert.Equal(t, runner.ModeOff, cfg.EffectiveMode())

	require.NoError(t, cfg.SetMode(runner.ModeClaude))
	assert.Equal(t, runner.ModeClaude, cfg.EffectiveMode())

	require.Error(t, cfg.SetMode("opencode"))
	assert.Equal(t, runner.ModeClaude, cfg.EffectiveMode())

	cfg.Mode = "bogus"
	assert.Equal(t, runner.ModeOff, cfg.EffectiveMode())
}

func TestMigrateModeTurnsOnAConfigThatNeverChoseOne(t *testing.T) {
	path := runner.ConfigPath(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"repos":["a/b"]}`), 0o600))

	changed, err := runner.MigrateMode(path)
	require.NoError(t, err)
	assert.True(t, changed)

	got, err := runner.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, runner.ModeClaude, got.EffectiveMode())
	assert.Equal(t, []string{"a/b"}, got.Repos)

	changed, err = runner.MigrateMode(path)
	require.NoError(t, err)
	assert.False(t, changed, "a second run changes nothing")
}

func TestMigrateModeKeepsAnExplicitOffAndMissingConfigs(t *testing.T) {
	path := runner.ConfigPath(t.TempDir())
	changed, err := runner.MigrateMode(path)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.NoFileExists(t, path, "no config is created for an unconfigured Mac")

	cfg, err := runner.LoadConfig(path)
	require.NoError(t, err)
	require.NoError(t, cfg.SetMode(runner.ModeOff))
	require.NoError(t, cfg.Save(path))

	changed, err = runner.MigrateMode(path)
	require.NoError(t, err)
	assert.False(t, changed)
	got, err := runner.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, runner.ModeOff, got.EffectiveMode())
}

func TestMigrateModeReportsAnUnreadableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
	_, err := runner.MigrateMode(path)
	require.Error(t, err)
}
