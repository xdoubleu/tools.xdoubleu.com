package runner_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// providerDownLine is the event Claude Code wrote when the job VM lost its route
// to the API mid-run (T-731's session, 2026-11-21).
const providerDownLine = `{"type":"assistant","error":"server_error",` +
	`"message":{"role":"assistant","content":[{"type":"text","text":` +
	`"API Error: No internet route — check your connection or VPN ` +
	`(EHOSTUNREACH)"}]},"session_id":"sess-9"}`

// providerDown is an agent run that died reaching its provider, committing
// nothing and writing no outcome.
func providerDown(_ int, spec runner.SandboxSpec) int {
	_, _ = fmt.Fprintln(spec.Log, providerDownLine)
	return 1
}

func TestAProviderOutageIsRetriedInsideTheSameAttempt(t *testing.T) {
	p := newPublishing(t, attempts(
		providerDown,
		agentRuns("Did it"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 2, "the lost route is ran again, not failed")
	assert.Equal(t, runner.ClaudeCode{}.Resume("sess-9"), p.sandbox.specs[1].Command)

	got := p.api.done()
	require.Len(t, got, 1)
	assert.True(t, got[0].succeeded)
	assert.Empty(t, got[0].blocked, "a retry that worked is no block")
	assert.Empty(t, runner.RetriesOf(got[0].end), "the board retry was not used")
	assert.Contains(t, p.api.noted(),
		"The model API was unreachable; running the same attempt again")
}

func TestAProviderOutageThatStaysReportsWhatTheHarnessSaid(t *testing.T) {
	p := newPublishing(t, attempts(providerDown))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 4, "bounded, not endless")
	got := p.api.done()
	require.Len(t, got, 1)
	assert.False(t, got[0].succeeded)
	assert.Contains(t, got[0].blocked, "The agent exited with status 1.")
	assert.Contains(t, got[0].blocked, "API Error: No internet route",
		"the block names the failure, so its report can be acted on")
	assert.Contains(t, got[0].blocked, "EHOSTUNREACH")
}
