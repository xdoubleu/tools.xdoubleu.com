package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// laterJob is a sandbox for the job after the first: it adds a commit and an outcome.
func laterJob(first func(context.Context, runner.SandboxSpec) (int, error)) func(
	context.Context, runner.SandboxSpec,
) (int, error) {
	runs := 0
	return func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		runs++
		if runs == 1 {
			return first(ctx, spec)
		}
		commitFile(spec.CloneDir, "more.txt", "more")
		_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"), []byte("More."), 0o644)
		return 0, nil
	}
}

func TestAReplyWithNoPushedBranchTakesOverTheClonePreviousJobLeft(t *testing.T) {
	env := newEnv(t, laterJob(writes("First.", 0)), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	env.api.jobs = append(env.api.jobs, replyJob())

	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)

	second := env.sandbox.specs[1]
	assert.FileExists(t, filepath.Join(second.CloneDir, "agent.txt"), "the earlier work")
	assert.Contains(t, second.Stdin, "taken over from an earlier job's clone")
	assert.Equal(t, runner.ClaudeCode{}.Resume("sess-9"), second.Command)
	assert.True(t, env.api.done()[1].succeeded, env.api.done()[1].outcome)
	assert.Contains(t, strings.Join(env.api.noted(), "\n"), "never pushed")
	assert.NoFileExists(t, filepath.Join(env.dir, "jobs", jobID, "clone", "agent.txt"),
		"the clone moved to the new job")
}

func TestARerunWithNoPushedBranchContinuesFromTheLocalClone(t *testing.T) {
	env := newEnv(t, laterJob(writes("First.", 0)), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	rerun := replyJob()
	rerun.ResumeSessionID = ""
	env.api.jobs = append(env.api.jobs, rerun)

	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)

	second := env.sandbox.specs[1]
	assert.Contains(t, second.Stdin, "continuing earlier work")
	assert.FileExists(t, filepath.Join(second.CloneDir, "agent.txt"))
	assert.True(t, env.api.done()[1].succeeded, env.api.done()[1].outcome)
}

func TestAReplyWithNoWorkAnywhereStartsFreshWithANote(t *testing.T) {
	env := newEnv(t, writes("ok", 0), replyJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.succeeded, got.outcome)
	require.Len(t, env.sandbox.specs, 1)
	assert.Contains(t, env.sandbox.specs[0].Stdin, "starts again from the base branch")
	assert.Contains(t, strings.Join(env.api.noted(), "\n"), "starts again")
}

func TestAJobWhosePreviousCloneHasADifferentBranchStartsFresh(t *testing.T) {
	other := newJob()
	other.Branch = "agent/other"
	env := newEnv(t, writes("First.", 0), other)
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	env.api.jobs = append(env.api.jobs, replyJob())

	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.sandbox.specs[1].Stdin, "starts again from the base branch")
}

func TestAJobThatEndsWithoutAPullRequestPushesItsBranchAnyway(t *testing.T) {
	p := newPublishing(t, writes("Did it.\n\nChecks: lint=fail, test=pass, docs=pass", 0))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.Empty(t, got.prURL)
	assert.NotEmpty(t, p.remoteBranch(t), "the work is on GitHub")
	assert.Contains(t, got.outcome, "on GitHub, without a pull request")
	assert.Empty(t, p.gh.opened)
}

func TestABlockedJobWithNothingToPushLeavesNoBranch(t *testing.T) {
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
			[]byte("Blocked: no network"), 0o644)
		return 0, nil
	})
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, p.remoteBranch(t), "no commits, nothing to keep")
	assert.NotContains(t, p.api.done()[0].outcome, "without a pull request")
}

func TestTheSandboxFixIsToldToBranchOffTheDefaultBranch(t *testing.T) {
	env := newEnv(t, writes("Did it.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.sandbox.specs[0].Stdin,
		"git switch -c agent/sandbox-<slug> origin/<the repository's default branch>")
}
