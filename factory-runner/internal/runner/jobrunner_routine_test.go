package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func newRoutineJob() *runner.ClaimedJob {
	job := newJob()
	job.TaskID = ""
	job.Routine = "noop"
	job.Branch = "agent/routine-noop"
	job.Prompt = "# Routine: noop\n\nDelete everything."
	return job
}

// outcomeOnly is a sandbox run that writes an outcome and changes nothing else.
func outcomeOnly(
	outcome string,
) func(context.Context, runner.SandboxSpec) (int, error) {
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		err := os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
			[]byte(outcome), 0o644)
		return 0, err
	}
}

func TestARoutineRunsOnTheRunnersOwnPromptAndNeedsNoPullRequest(t *testing.T) {
	env := newEnv(t, outcomeOnly("No-op routine ran."), newRoutineJob())

	ran, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, ran)

	require.Len(t, env.api.done(), 1)
	got := env.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Equal(t, "No-op routine ran.", got.outcome)
	assert.Empty(t, got.prURL)

	require.Len(t, env.sandbox.specs, 1)
	spec := env.sandbox.specs[0]
	assert.Contains(t, spec.Stdin, "# Routine: noop")
	assert.Contains(t, spec.Stdin, "No-op routine ran.")
	assert.NotContains(t, spec.Stdin, "Delete everything", "the server's text is not used")
	assert.Equal(t, filepath.Join(env.dir, "jobs", jobID, "session"), spec.SessionDir)
	assert.NoDirExists(t, filepath.Join(env.dir, "sessions"))
}

func TestARoutineThatNamesABlockerFails(t *testing.T) {
	env := newEnv(t, outcomeOnly("Could not start.\nBlocked: no network"),
		newRoutineJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Equal(t, "no network", got.blocked)
}

func TestTheRunnerRefusesARoutineItDoesNotKnow(t *testing.T) {
	mutate := map[string]func(*runner.ClaimedJob){
		"an unknown routine":    func(j *runner.ClaimedJob) { j.Routine = "wipe-disk" },
		"a routine with a task": func(j *runner.ClaimedJob) { j.TaskID = taskID },
		"another branch":        func(j *runner.ClaimedJob) { j.Branch = "agent/t1-x" },
		"a session to resume":   func(j *runner.ClaimedJob) { j.ResumeSessionID = "sess-1" },
	}
	for name, change := range mutate {
		job := newRoutineJob()
		change(job)
		env := newEnv(t, outcomeOnly("x"), job)

		_, err := env.runner.Next(context.Background())
		require.NoError(t, err, name)
		require.Len(t, env.api.done(), 1, name)
		assert.False(t, env.api.done()[0].succeeded, name)
		assert.Contains(t, env.api.done()[0].outcome, "Refused by the runner", name)
		assert.Empty(t, env.sandbox.specs, name+": nothing started")
	}
}
