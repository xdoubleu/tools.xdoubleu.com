package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func useCLIDirs(t *testing.T, dirs ...string) {
	t.Helper()
	old := runner.CLIDirs
	runner.CLIDirs = func(string) []string { return dirs }
	t.Cleanup(func() { runner.CLIDirs = old })
}

func TestLinkCLILinksIntoTheFirstWritableDir(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	good := filepath.Join(root, "bin")
	useCLIDirs(t, filepath.Join(blocked, "bin"), good)

	link, err := runner.LinkCLI(root, "/app/factory-runner")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(good, "factory-runner"), link)
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, "/app/factory-runner", target)
}

func TestLinkCLIRepointsAnOldLinkAndKeepsARegularFile(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, os.MkdirAll(first, 0o755))
	require.NoError(t, os.MkdirAll(second, 0o755))
	useCLIDirs(t, first, second)
	require.NoError(t, os.Symlink("/old", filepath.Join(first, "factory-runner")))

	_, err := runner.LinkCLI(root, "/new")
	require.NoError(t, err)
	target, _ := os.Readlink(filepath.Join(first, "factory-runner"))
	assert.Equal(t, "/new", target)
	_, err = runner.LinkCLI(root, "/new") // already right: no-op
	require.NoError(t, err)

	own := filepath.Join(second, "factory-runner")
	require.NoError(t, os.Remove(filepath.Join(first, "factory-runner")))
	mine := filepath.Join(first, "factory-runner")
	require.NoError(t, os.WriteFile(mine, []byte("x"), 0o755))
	require.NoError(t, os.WriteFile(own, []byte("y"), 0o755))
	_, err = runner.LinkCLI(root, "/new")
	require.Error(t, err)
}
