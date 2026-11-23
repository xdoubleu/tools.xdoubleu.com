package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestABlockedLineInTheOutcomeIsReported(t *testing.T) {
	env := newEnv(t, writes("Did it.\n\nBlocked: no Go module cache\n", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "no Go module cache", env.api.done()[0].blocked)
}

func TestNoBlockedLineOrNoneReportsNothing(t *testing.T) {
	for _, outcome := range []string{"Did it.", "Did it.\nBlocked: none"} {
		env := newEnv(t, writes(outcome, 0), newJob())
		_, err := env.runner.Next(context.Background())
		require.NoError(t, err)
		assert.Empty(t, env.api.done()[0].blocked, outcome)
	}
}

func TestAJobThatEndsWithoutAnOutcomeIsReportedAsBlocked(t *testing.T) {
	env := newEnv(t, writes("", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.api.done()[0].blocked, "wrote no outcome file")

	env = newEnv(t, writes("", 3), newJob())
	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, env.api.done()[0].blocked, "a crash is a failed job, not a gap")
	assert.Contains(t, env.api.done()[0].outcome, "exited with status 3")

	env = newEnv(t, func(context.Context, runner.SandboxSpec) (int, error) {
		return 0, errors.New("vm gone")
	}, newJob())
	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.api.done()[0].blocked, "vm gone")
}

func TestAnAgentsOwnBlockedLineWinsOverTheFallback(t *testing.T) {
	env := newEnv(t, writes("Blocked: no network", 3), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "no network", env.api.done()[0].blocked)
}

func TestThePreambleNamesTheGatesTheChecksAndBlockedLines(t *testing.T) {
	env := newEnv(t, writes("Did it.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	stdin := env.sandbox.specs[0].Stdin
	assert.Contains(t, stdin, "Checks: lint=pass")
	assert.Contains(t, stdin, "Blocked: <the problem>")
	assert.Contains(t, stdin, "factory-sandbox")
}

func TestAWaitingOnLineNamesTheTasksAndFilesNoBlocker(t *testing.T) {
	for _, line := range []string{"Waiting-on: T-6, T-8 and T-6", "Blocked-by: T-6, T-8"} {
		env := newEnv(t, writes("Did none.\n\n"+line+"\n", 0), newJob())
		_, err := env.runner.Next(context.Background())
		require.NoError(t, err)
		got := env.api.done()[0]
		assert.Equal(t, []string{"T-6", "T-8"}, got.end.WaitingOn, line)
		assert.Empty(t, got.blocked, line)
	}
}

func TestNoWaitingOnLineWaitsOnNothing(t *testing.T) {
	env := newEnv(t, writes("Did it.\n\nWaiting-on: none\n", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, env.api.done()[0].end.WaitingOn)
}

func TestAWaitingOnJobStopsTheLoopAtOnce(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns(
		"Could not start\n\nChecks: lint=pass, test=fail\nWaiting-on: T-6", true)))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 1)
	assert.Equal(t, []string{"T-6"}, p.api.done()[0].end.WaitingOn)
}

func TestThePreambleExplainsWaitingOnAgainstBlocked(t *testing.T) {
	env := newEnv(t, writes("Did it.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.sandbox.specs[0].Stdin, "Waiting-on: T-6, T-8")
}

func TestAnAlreadyFixedJobEndsAtOnceWithoutGatesOrAPullRequest(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns(
		"Did none\n\nAlready-fixed: go build ./... passes", true)))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 1)
	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Empty(t, got.blocked)
}

func TestThePreambleOffersAlreadyFixedForAProblemThatNoLongerReproduces(t *testing.T) {
	env := newEnv(t, writes("Did it.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.sandbox.specs[0].Stdin, "Already-fixed: <evidence>")
}

func TestThePreambleSplitsATaskTooBigForOneJob(t *testing.T) {
	env := newEnv(t, writes("Did it.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.sandbox.specs[0].Stdin, "check the task fits one job")
	assert.Contains(t, env.sandbox.specs[0].Stdin, "epic:<slug>")
}
