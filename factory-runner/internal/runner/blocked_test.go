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
	assert.Contains(t, env.api.done()[0].blocked, "exited with status 3")

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
