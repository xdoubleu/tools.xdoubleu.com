package ward_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

func TestHostGitRunsGitInTheDirectory(t *testing.T) {
	out, err := ward.HostGit(context.Background(), t.TempDir(), "--version")
	require.NoError(t, err)
	assert.Contains(t, out, "git version")
	_, err = ward.HostGit(context.Background(), t.TempDir(), "rev-parse",
		"--show-toplevel")
	require.Error(t, err, "not a repository")
}

func TestHostGHRunsGHInTheDirectory(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = fail ]; then exit 1; fi\npwd\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o700))
	t.Setenv("PATH", bin)
	dir := t.TempDir()
	out, err := ward.HostGH(context.Background(), dir, "pr")
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(dir), filepath.Base(strings.TrimSpace(out)))
	_, err = ward.HostGH(context.Background(), dir, "fail")
	require.Error(t, err)
}

func TestHostGHWithoutGHFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ward.HostGH(context.Background(), t.TempDir(), "pr")
	require.Error(t, err)
}
