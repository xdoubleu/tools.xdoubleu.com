package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		RunWithEnv: nil,
	}
}

func TestCloneNamesTheGitStepThatFailed(t *testing.T) {
	// First clone of a repo: mirror, clone, set-url, checkout.
	for step, want := range []string{"clone", "", "remote", "checkout"} {
		if want == "" {
			continue // a failed job clone is retried, see the rebuild tests
		}
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

func TestRestoreNamesTheGitStepThatFailed(t *testing.T) {
	// A mirror that is already there: update, clone, set-url, rev-parse.
	for step, want := range []string{"remote", "clone", "remote", "rev-parse"} {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "mirrors", "o", "n.git"), 0o755))
		var calls []string
		g := failingGit(dir, step+1, &calls)
		// Updating the mirror failing falls back to cloning it afresh, which is a
		// further call; only the later steps are asserted by name.
		_, err := g.Restore(
			context.Background(), "o", "n", "agent/x", filepath.Join(t.TempDir(), "d"),
		)
		if step <= 1 {
			require.NoError(t, err, "a failed update or clone is retried")
			continue
		}
		require.Error(t, err, want)
		assert.Contains(t, err.Error(), "git "+want)
	}
}

func TestRestoreReturnsTheBranchTip(t *testing.T) {
	g := &runner.Git{
		Dir:       t.TempDir(),
		RemoteURL: func(string, string) string { return "https://example.com/o/n.git" },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if args[0] == "rev-parse" {
				return []byte("abc123\n"), nil
			}
			return nil, nil
		},
		RunWithEnv: nil,
	}
	tip, err := g.Restore(
		context.Background(), "o", "n", "agent/x", filepath.Join(t.TempDir(), "d"),
	)
	require.NoError(t, err)
	assert.Equal(t, "abc123", tip)
}

func TestAFailedJobCloneRebuildsTheMirrorAndRetriesOnce(t *testing.T) {
	dir := t.TempDir()
	mirror := filepath.Join(dir, "mirrors", "o", "n.git")
	dest := filepath.Join(t.TempDir(), "d")

	// remote update ok, job clone fails, then mirror clone and job clone.
	require.NoError(t, os.MkdirAll(mirror, 0o755))
	var calls []string
	g := failingGit(dir, 2, &calls)
	require.NoError(t, g.Clone(context.Background(), "o", "n", "main", "agent/x", dest))
	assert.Equal(t,
		[]string{"remote", "clone", "clone", "clone", "remote", "checkout"}, calls)

	// A second failure is returned.
	require.NoError(t, os.MkdirAll(mirror, 0o755))
	calls = nil
	g = failingGit(dir, 2, &calls)
	first := g.Run
	g.Run = func(c context.Context, d string, a ...string) ([]byte, error) {
		out, err := first(c, d, a...)
		if err == nil && len(calls) == 4 {
			return []byte("again"), errors.New("exit 128")
		}
		return out, err
	}
	err := g.Clone(context.Background(), "o", "n", "main", "agent/x", dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "again")

	// Restore retries too.
	require.NoError(t, os.MkdirAll(mirror, 0o755))
	calls = nil
	g = failingGit(dir, 2, &calls)
	_, err = g.Restore(context.Background(), "o", "n", "agent/x", dest)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"remote", "clone", "clone", "clone", "remote", "rev-parse"}, calls)
}

func TestARebuildThatFailsIsReturnedAndAnUnopenableLockFails(t *testing.T) {
	dir := t.TempDir()
	mirror := filepath.Join(dir, "mirrors", "o", "n.git")
	require.NoError(t, os.MkdirAll(mirror, 0o755))
	n := 0
	g := &runner.Git{
		Dir:       dir,
		RemoteURL: func(string, string) string { return "x" },
		Run: func(context.Context, string, ...string) ([]byte, error) {
			n++
			if n >= 2 { // the job clone, then the mirror's re-clone
				return []byte("nope"), errors.New("exit 128")
			}
			return nil, nil
		},
		RunWithEnv: nil,
	}
	err := g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")

	// The lock path is a directory, so it can't be opened.
	require.NoError(t, os.RemoveAll(mirror+".lock"))
	require.NoError(t, os.MkdirAll(mirror+".lock", 0o755))
	err = g.Clone(context.Background(), "o", "n", "main", "agent/x",
		filepath.Join(t.TempDir(), "d"))
	require.ErrorContains(t, err, "mirror lock")
	_, err = g.Restore(context.Background(), "o", "n", "agent/x",
		filepath.Join(t.TempDir(), "d"))
	require.ErrorContains(t, err, "mirror lock")
}

func TestCloneRepairsAMirrorWhoseRefPointsAtAMissingObject(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin")
	require.NoError(t, os.MkdirAll(origin, 0o755))
	gitIn(t, origin, "init", "-b", "main")
	gitIn(t, origin, "commit", "--allow-empty", "-m", "one")
	tip := gitIn(t, origin, "rev-parse", "HEAD")

	dir := t.TempDir()
	mirror := filepath.Join(dir, "mirrors", "o", "n.git")
	require.NoError(t, os.MkdirAll(filepath.Dir(mirror), 0o755))
	gitIn(t, dir, "clone", "--mirror", "--", origin, mirror)
	ref := filepath.Join(mirror, "refs", "heads", "main")
	require.NoError(t, os.MkdirAll(filepath.Dir(ref), 0o755))
	require.NoError(t,
		os.WriteFile(ref, []byte(strings.Repeat("a", len(tip))+"\n"), 0o600))

	g := &runner.Git{
		Dir:        dir,
		RemoteURL:  func(string, string) string { return origin },
		Run:        nil,
		RunWithEnv: nil,
	}
	dest := filepath.Join(t.TempDir(), "d")
	require.NoError(t, g.Clone(context.Background(), "o", "n", "main", "agent/x", dest))
	assert.Equal(t, tip, gitIn(t, dest, "rev-parse", "HEAD"))
}
