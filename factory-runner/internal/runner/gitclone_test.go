package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// failingGit fails its failAt-th call (1-based) and records every call's first
// argument.
func failingGit(dir string, failAt int, calls *[]string) *runner.Git {
	return &runner.Git{
		Dir:       dir,
		RemoteURL: func(string, string) string { return "https://example.com/o/n.git" },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			*calls = append(*calls, args[0])
			if len(*calls) == failAt {
				return []byte("boom"), errors.New("exit 128")
			}
			return nil, nil
		},
	}
}

func TestCloneNamesTheGitStepThatFailed(t *testing.T) {
	// First clone of a repo: mirror, clone, set-url, checkout.
	for step, want := range []string{"clone", "clone", "remote", "checkout"} {
		var calls []string
		g := failingGit(t.TempDir(), step+1, &calls)
		err := g.Clone(
			context.Background(),
			"o",
			"n",
			"main",
			"agent/x",
			filepath.Join(t.TempDir(), "d"),
		)
		require.Error(t, err, want)
		assert.Contains(t, err.Error(), "git "+want)
		assert.Contains(t, err.Error(), "boom")
		assert.Len(t, calls, step+1, "it stops at the first failure")
	}
}

func TestCloneUpdatesAnExistingMirrorFirst(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mirrors", "o", "n.git"), 0o755))

	var calls []string
	g := failingGit(dir, 0, &calls)
	require.NoError(t, g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d")))
	assert.Equal(t, []string{"remote", "clone", "remote", "checkout"}, calls)

	// A mirror that can't be updated is cloned afresh, once.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mirrors", "o", "n.git"), 0o755))
	calls = nil
	g = failingGit(dir, 1, &calls)
	require.NoError(t, g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d")))
	assert.Equal(t, []string{"remote", "clone", "clone", "remote", "checkout"}, calls)

	// And if that clone fails too, the error says so.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mirrors", "o", "n.git"), 0o755))
	calls = nil
	g = failingGit(dir, 2, &calls)
	err := g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone")
}

func TestCloneCannotCreateItsDirectories(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	var calls []string

	// The mirror directory is under a file.
	g := failingGit(blocker, 0, &calls)
	require.Error(t, g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d")))
	assert.Empty(t, calls)

	// The job directory is under a file.
	g = failingGit(t.TempDir(), 0, &calls)
	require.Error(t, g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(blocker, "d")))
}

func TestLoopReturnsWhenItsContextIsAlreadyDone(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, env.runner.Loop(ctx))
}

func TestAJobWhoseOutcomeDirectoryCannotBeMadeFails(t *testing.T) {
	env := newEnv(t, writes("ok", 0), newJob())
	// A clone that leaves a file where the outcome directory goes.
	env.runner.Git.Run = func(
		_ context.Context, _ string, args ...string,
	) ([]byte, error) {
		if args[0] == "checkout" {
			path := filepath.Join(env.dir, "jobs", jobID, "outcome")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "outcome directory")
	assert.Empty(t, env.sandbox.specs)
}
