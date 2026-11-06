package runner_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestAClaudeTokenIsNeverPutInAVMThatHasNoProxy(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.ClaudeToken = func() (string, error) { return claudeToken, nil }
	env.runner.Proxy = nil

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "needs the egress proxy")
	assert.Empty(t, env.sandbox.specs, "the sandbox never started")
	assert.NotContains(t, got.outcome, claudeToken)
}

func TestTheClaudeTokenIsReadOnceAndThatValueIsWhatIsRedacted(t *testing.T) {
	reads := 0
	env := newEnv(t, writes("Used "+claudeToken+" there.", 0), newJob())
	env.runner.ClaudeToken = func() (string, error) {
		reads++
		if reads > 1 {
			// A later read that fails must not let the first value slip through.
			return "", assert.AnError
		}
		return claudeToken, nil
	}
	env.runner.Proxy = proxyOK

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, reads)
	got := env.api.done()[0]
	assert.Contains(t, got.outcome, "the outcome contains a credential value")
	assert.NotContains(t, got.outcome, claudeToken)
}

func TestACommitDefaultsToAnAddressThatBelongsToNoAccount(t *testing.T) {
	assert.Equal(t, "factory-bot@users.invalid", runner.DefaultBotEmail)
	assert.NotContains(t, runner.DefaultBotEmail, "noreply.github.com")
}
