package runner_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const leakedSecret = "sk-ant-oat01-not-a-real-secret"

// agentDoes is a sandbox run that lets do change the clone, then writes an outcome.
func agentDoes(
	outcome string,
	do func(clone string),
) func(context.Context, runner.SandboxSpec) (int, error) {
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"s"}`+"\n")
		do(spec.CloneDir)
		_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"), []byte(outcome), 0o644)
		return 0, nil
	}
}

// finish runs the one queued job and returns how it was reported.
func finish(t *testing.T, env *jobEnv) completion {
	t.Helper()
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	done := env.api.done()
	require.Len(t, done, 1)
	return done[0]
}

func TestAJobWithNoCommitsIsNotPublished(t *testing.T) {
	env := newEnv(t, agentDoes("Thought about it.", func(string) {}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Not published: the agent made no commits")
	assert.Contains(t, got.outcome, "Thought about it.", "the agent's own words stay")
}

func TestAJobThatTouchesWorkflowsIsNotPublished(t *testing.T) {
	for _, path := range []string{
		".github/workflows/ci.yml",
		".GitHub/Workflows/ci.yml",
		".github/workflows",
	} {
		env := newEnv(t, agentDoes("done", func(clone string) {
			commitFile(clone, path, "on: push")
		}), newJob())
		got := finish(t, env)
		assert.False(t, got.succeeded, path)
		assert.Contains(t, got.outcome, "may not touch", path)
	}
}

func TestOtherGithubFilesAreFine(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, ".github/CODEOWNERS", "* @x")
		commitFile(clone, ".github/workflows-notes.md", "x")
	}), newJob())
	assert.True(t, finish(t, env).succeeded)
}

func TestASecretInTheWorkIsNotPublishedAndNotRepeated(t *testing.T) {
	cases := map[string]func(clone string){
		"in a file": func(clone string) { commitFile(clone, "a.txt", "key="+leakedSecret) },
		"committed then removed": func(clone string) {
			commitFile(clone, "a.txt", leakedSecret)
			commitFile(clone, "a.txt", "clean")
		},
		"in a commit message": func(clone string) {
			commitFile(clone, "a.txt", "clean")
			cmd := exec.Command("git", "-c", "user.name=a", "-c", "user.email=a@x.io",
				"commit", "--allow-empty", "-m", "uses "+leakedSecret)
			cmd.Dir = clone
			_ = cmd.Run()
		},
	}
	for name, do := range cases {
		env := newEnv(t, agentDoes("done", do), newJob())
		env.runner.Secrets = []string{leakedSecret}
		got := finish(t, env)
		assert.False(t, got.succeeded, name)
		assert.Contains(t, got.outcome, "a credential value appears in", name)
		assert.NotContains(t, got.outcome, leakedSecret, name)
	}
}

func TestTheDeviceTokenIsAlwaysASecretAndShortValuesAreIgnored(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "tok-device-12345")
	}), newJob())
	env.runner.Token = "tok-device-12345"
	assert.False(t, finish(t, env).succeeded)

	env = newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "abc")
	}), newJob())
	env.runner.Secrets = []string{"abc", ""}
	assert.True(t, finish(t, env).succeeded, "too short to search for")
}

func TestASecretInTheOutcomeIsNotReported(t *testing.T) {
	env := newEnv(t, agentDoes("my key is "+leakedSecret, func(clone string) {
		commitFile(clone, "a.txt", "clean")
	}), newJob())
	env.runner.Secrets = []string{leakedSecret}
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "the outcome contains a credential value")
	assert.NotContains(t, got.outcome, leakedSecret)
}

func TestACloneWhoseGitDirIsAPointerIsRefused(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "x")
		_ = os.RemoveAll(filepath.Join(clone, ".git"))
		_ = os.WriteFile(filepath.Join(clone, ".git"), []byte("gitdir: "+clone), 0o644)
	}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "not a plain directory")
}

func TestACloneThatBorrowsObjectsIsRefused(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "x")
		_ = os.WriteFile(
			filepath.Join(clone, ".git", "objects", "info", "alternates"),
			[]byte("/elsewhere/objects\n"), 0o644)
	}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "borrows objects")
}

func TestAJobThatLeftItsBranchIsNotPublished(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		cmd := exec.Command("git", "checkout", "-q", "-B", "elsewhere")
		cmd.Dir = clone
		_ = cmd.Run()
		commitFile(clone, "a.txt", "x")
		cmd = exec.Command("git", "branch", "-D", branch)
		cmd.Dir = clone
		_ = cmd.Run()
	}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "could not read the branch")
}

func TestARejectedJobLeavesNothingInTheMirrorOrTheReviewDir(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", leakedSecret)
	}), newJob())
	env.runner.Secrets = []string{leakedSecret}
	require.NoError(t, os.MkdirAll(
		filepath.Join(env.dir, "mirrors", "xdoubleu"), 0o755))
	mirror := filepath.Join(env.dir, "mirrors", "xdoubleu", "tools.xdoubleu.com.git")
	gitIn(t, env.dir, "clone", "--mirror", "--", env.remote, mirror)
	count := func() string {
		return gitIn(t, mirror, "cat-file", "--batch-all-objects", "--batch-check")
	}
	before := count()

	assert.False(t, finish(t, env).succeeded)
	assert.Equal(t, before, count(), "the agent's objects never reach the mirror")
	assert.NoDirExists(t, filepath.Join(env.dir, "reviews", jobID+".git"))
}

func TestAWorkflowChangeThatIsRevertedStillCounts(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "x")
		commitFile(clone, ".github/workflows/x.yml", "on: push")
		cmd := exec.Command("git", "rm", "-q", ".github/workflows/x.yml")
		cmd.Dir = clone
		_ = cmd.Run()
		cmd = exec.Command("git", "-c", "user.name=a", "-c", "user.email=a@x.io",
			"commit", "-q", "-m", "undo")
		cmd.Dir = clone
		_ = cmd.Run()
	}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "may not touch")
}

func TestASecretInAnAuthorNameOrAMergeIsFound(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		cmd := exec.Command("git", "-c", "user.name="+leakedSecret, "-c", "user.email=a@x.io",
			"commit", "-q", "--allow-empty", "-m", "hello")
		cmd.Dir = clone
		_ = cmd.Run()
		commitFile(clone, "a.txt", "x")
	}), newJob())
	env.runner.Secrets = []string{leakedSecret}
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "message or author")

	// A merge whose own resolution carries the secret.
	env = newEnv(t, agentDoes("done", func(clone string) {
		run := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-c", "user.name=a",
				"-c", "user.email=a@x.io"}, args...)...)
			cmd.Dir = clone
			_ = cmd.Run()
		}
		commitFile(clone, "a.txt", "one")
		run("checkout", "-q", "-b", "side", "HEAD~1")
		commitFile(clone, "b.txt", "two")
		run("checkout", "-q", branch)
		run("merge", "--no-ff", "--no-commit", "side")
		_ = os.WriteFile(filepath.Join(clone, "c.txt"), []byte(leakedSecret), 0o644)
		run("add", "-A")
		run("commit", "-q", "-m", "merge")
	}), newJob())
	env.runner.Secrets = []string{leakedSecret}
	got = finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "a credential value appears in commit")
}

func TestCommitsThatChangeNothingAreNotPublished(t *testing.T) {
	empty := func(clone string) {
		cmd := exec.Command("git", "-c", "user.name=a", "-c", "user.email=a@x.io",
			"commit", "-q", "--allow-empty", "-m", "nothing")
		cmd.Dir = clone
		_ = cmd.Run()
	}
	env := newEnv(t, agentDoes("done", empty), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "change nothing")

	env = newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "x")
		cmd := exec.Command("git", "reset", "-q", "--hard", "HEAD~1")
		cmd.Dir = clone
		_ = cmd.Run()
		empty(clone)
	}), newJob())
	assert.False(t, finish(t, env).succeeded)
}

func TestACloneWithASymlinkedObjectsDirIsRefused(t *testing.T) {
	env := newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", "x")
		objects := filepath.Join(clone, ".git", "objects")
		_ = os.Rename(objects, objects+".real")
		_ = os.Symlink(objects+".real", objects)
	}), newJob())
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "not a plain directory")
}
