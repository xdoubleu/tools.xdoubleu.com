package runner_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestAJobThatNeededRetriesEndsWithOneBoundedRetroInItsSession(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("First try\n\nChecks: lint=pass, test=fail", true),
		agentRuns("Second try"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 3)
	retro := p.sandbox.specs[2]
	assert.Equal(t, runner.ClaudeCode{}.Resume("sess-9"), retro.Command)
	assert.Contains(t, retro.Stdin, "factory-retro")
	assert.Contains(t, retro.Stdin, "test fail")
	assert.Contains(t, retro.Stdin, "Phase times:")
	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Equal(t, "Second try"+passAll, got.outcome)
}

func TestAJobThatNeededNoRetryGetsNoRetro(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Done"+passAll, true)))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 1)
}
