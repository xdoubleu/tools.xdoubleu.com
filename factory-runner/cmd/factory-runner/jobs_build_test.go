package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// versionExec answers `container --version`.
type versionExec struct {
	out string
	err error
}

func (v versionExec) Exec(
	_ context.Context,
	_ []string,
	_ io.Reader,
	out io.Writer,
) (int, error) {
	_, _ = io.WriteString(out, v.out)
	if v.err != nil {
		return -1, v.err
	}
	return 0, nil
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestTheJobRunnerIsBuiltFromTheConfigOnAMacThatCanRunJobs(t *testing.T) {
	e := newEnv(t)
	cfg := runner.Config{
		Server: "https://example.com/api", Repos: []string{"a/b"}, Image: "job:v2",
		DataDir: filepath.Join(e.app.home, "data"), GoModCache: "/c/go", NPMCache: "/c/npm",
	}
	require.NoError(t, cfg.Save(runner.ConfigPath(e.app.home)))

	jr, err := newJobRunner(context.Background(), e.app, cfg, "frt_tok", quietLog(),
		versionExec{out: "container CLI version 1.6.1", err: nil})
	require.NoError(t, err)

	assert.Equal(t, "frt_tok", jr.Token)
	assert.Equal(t, cfg.DataDir, jr.DataDir)
	assert.DirExists(t, cfg.DataDir)
	assert.Equal(t, "https://example.com/api", jr.ServerURL)
	assert.Equal(t, []string{"frt_tok"}, jr.Secrets)
	assert.Same(t, e.app.activity, jr.Activity)
	box, ok := jr.Sandbox.(*runner.ContainerSandbox)
	require.True(t, ok)
	assert.Equal(t, "job:v2", box.Image)
	assert.Equal(t, "/c/go", box.GoModCache)
	assert.Equal(t, "/c/npm", box.NPMCache)
	repos, err := jr.Allowed()
	require.NoError(t, err)
	assert.Equal(t, []string{"a/b"}, repos)

	// `config repos add` takes effect on the next claim, without a restart.
	require.Equal(t, 0, e.do("config", "repos", "add", "c/d"))
	repos, err = jr.Allowed()
	require.NoError(t, err)
	assert.Equal(t, []string{"a/b", "c/d"}, repos)
}

func TestAMacWithoutTheRightContainerCLICannotBuildTheJobRunner(t *testing.T) {
	e := newEnv(t)
	cfg := runner.Config{
		Server: runner.DefaultServer, Repos: nil, Image: "", DataDir: "",
		GoModCache: "", NPMCache: "",
	}
	for name, exec := range map[string]versionExec{
		"missing":      {out: "", err: errors.New("container: not found")},
		"wrong series": {out: "container CLI version 2.0.0", err: nil},
	} {
		jr, err := newJobRunner(context.Background(), e.app, cfg, "t", quietLog(), exec)
		require.ErrorContains(t, err, "jobs cannot run on this Mac", name)
		assert.Nil(t, jr, name)
	}
}

func TestADataDirectoryThatCannotBeCreatedFailsTheBuild(t *testing.T) {
	e := newEnv(t)
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	cfg := runner.Config{
		Server: runner.DefaultServer, Repos: nil, Image: "", GoModCache: "", NPMCache: "",
		DataDir: filepath.Join(blocker, "d"),
	}

	_, err := newJobRunner(context.Background(), e.app, cfg, "t", quietLog(),
		versionExec{out: "container CLI version 1.6.2", err: nil})
	require.ErrorContains(t, err, "data directory")
}
