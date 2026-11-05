package runner_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func emptyConfig() runner.Config {
	return runner.Config{
		Server: "", Repos: nil, Image: "", DataDir: "", GoModCache: "", NPMCache: "",
	}
}

func TestTheImageAndDataDirectoryDefaultUntilConfigured(t *testing.T) {
	c := emptyConfig()
	assert.Equal(t, runner.DefaultImage, c.JobImage())
	assert.Equal(t, runner.DefaultDataDir("/Users/x"), c.DataPath("/Users/x"))
	assert.Equal(t,
		filepath.Join("/Users/x", "Library", "Application Support", "factory-runner", "data"),
		c.DataPath("/Users/x"))

	require.NoError(t, c.SetImage("registry.example/job:v3@sha256:abc"))
	require.NoError(t, c.SetDir("data", "/Volumes/ssd//factory/"))
	assert.Equal(t, "registry.example/job:v3@sha256:abc", c.JobImage())
	assert.Equal(t, "/Volumes/ssd/factory", c.DataPath("/Users/x"))
}

func TestAnImageReferenceThatCouldBeReadAsAFlagIsRefused(t *testing.T) {
	c := emptyConfig()
	for _, ref := range []string{"", "-v", "--privileged", "a b", "a;b", "a\nb"} {
		require.Error(t, c.SetImage(ref), ref)
	}
	assert.Empty(t, c.Image)
}

func TestDirectoriesMustBeAbsoluteAndMountable(t *testing.T) {
	c := emptyConfig()
	require.NoError(t, c.SetDir("gocache", "/cache/go"))
	require.NoError(t, c.SetDir("npmcache", "/cache/npm"))
	assert.Equal(t, "/cache/go", c.GoModCache)
	assert.Equal(t, "/cache/npm", c.NPMCache)

	for _, dir := range []string{"", "relative", "/a,b", "/a\nb", "/a\rb"} {
		require.Error(t, c.SetDir("data", dir), dir)
	}
	require.ErrorContains(t, c.SetDir("other", "/x"), "unknown directory")
	assert.Empty(t, c.DataDir)
}
