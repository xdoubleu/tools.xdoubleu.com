package runner_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func writeTool(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	script := "#!/bin/sh\necho found-in-" + filepath.Base(dir) + "\n"
	path := filepath.Join(dir, "factory-fake-tool")
	require.NoError(t, os.WriteFile(path, []byte(script), mode))
}

func TestOSExecerFindsACommandOutsidePathInTheFallbackDirs(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	missing := t.TempDir()
	fallback := filepath.Join(t.TempDir(), "brew")
	require.NoError(t, os.Mkdir(fallback, 0o755))
	writeTool(t, fallback, 0o755)

	var out bytes.Buffer
	code, err := runner.OSExecer{BinDirs: []string{missing, fallback}}.Exec(
		context.Background(), []string{"factory-fake-tool"}, nil, &out)

	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Equal(t, "found-in-brew\n", out.String())
}

func TestOSExecerPrefersPathOverTheFallbackDirs(t *testing.T) {
	onPath := filepath.Join(t.TempDir(), "path")
	fallback := filepath.Join(t.TempDir(), "brew")
	require.NoError(t, os.Mkdir(onPath, 0o755))
	require.NoError(t, os.Mkdir(fallback, 0o755))
	writeTool(t, onPath, 0o755)
	writeTool(t, fallback, 0o755)
	t.Setenv("PATH", onPath+":/usr/bin:/bin")

	var out bytes.Buffer
	_, err := runner.OSExecer{BinDirs: []string{fallback}}.Exec(
		context.Background(), []string{"factory-fake-tool"}, nil, &out)

	require.NoError(t, err)
	assert.Equal(t, "found-in-path\n", out.String())
}

func TestOSExecerIgnoresANonExecutableFallbackAndNamesTheCommand(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	fallback := t.TempDir()
	writeTool(t, fallback, 0o644)

	code, err := runner.OSExecer{BinDirs: []string{fallback}}.Exec(
		context.Background(), []string{"factory-fake-tool"}, nil, &bytes.Buffer{})

	assert.Equal(t, -1, code)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "factory-fake-tool")
}
