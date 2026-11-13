package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const claudeToken = "sk-ant-oat01-a-long-enough-setup-token-value"

func proxyOK(*runner.ClaimedJob) (runner.ProxyListener, error) {
	return &fakeProxy{gateway: new(string), err: nil, port: nil, onStop: func() {}}, nil
}

func TestAClaudeJobGetsItsTokenItsAuthorAndItsOwnProxy(t *testing.T) {
	job := newJob()
	job.MCPClientID = "agent-0123abcd"
	env := newEnv(t, writes("Done.", 0), job)
	env.runner.ClaudeToken = func() (string, error) { return claudeToken, nil }
	env.runner.AuthorEmail = "123+factory[bot]@users.noreply.github.com"
	var proxiedFor string
	env.runner.Proxy = func(j *runner.ClaimedJob) (runner.ProxyListener, error) {
		proxiedFor = j.ID
		return proxyOK(j)
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	spec := env.sandbox.specs[0]
	assert.Equal(t, claudeToken, spec.Env["CLAUDE_CODE_OAUTH_TOKEN"])
	assert.Equal(t, "agent-0123abcd", spec.Env["GIT_AUTHOR_NAME"])
	assert.Equal(t, "agent-0123abcd", spec.Env["GIT_COMMITTER_NAME"])
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		assert.Equal(t, "123+factory[bot]@users.noreply.github.com", spec.Env[k], k)
	}
	assert.Equal(t, jobID, proxiedFor)
	assert.NotNil(t, spec.Proxy)
}

func TestACommitAuthorFallsBackWhenTheJobHasNoAccountOrEmailIsUnset(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	spec := env.sandbox.specs[0]
	assert.Equal(t, "factory", spec.Env["GIT_AUTHOR_NAME"])
	assert.Equal(t, runner.DefaultBotEmail, spec.Env["GIT_AUTHOR_EMAIL"])
	assert.NotContains(t, spec.Env, "CLAUDE_CODE_OAUTH_TOKEN", "no token source, none set")
	assert.Nil(t, spec.Proxy)
}

func TestAnOpenCodeJobDoesNotGetTheClaudeToken(t *testing.T) {
	job := newJob()
	job.Harness, job.Model = runner.HarnessOpenCode, "vendor/model"
	env := newEnv(t, writesOpenCode("Done."), job)
	env.runner.ClaudeToken = func() (string, error) { return claudeToken, nil }
	env.runner.Harness = runner.OpenCode{Model: "vendor/model"}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, env.sandbox.specs[0].Env, "CLAUDE_CODE_OAUTH_TOKEN")
}

func TestAMissingClaudeTokenFailsTheJobBeforeTheSandboxStarts(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.ClaudeToken = func() (string, error) {
		return "", errors.New("secret not stored: claude")
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Could not prepare the job")
	assert.Contains(t, got.outcome, "secret not stored: claude")
	assert.Empty(t, env.sandbox.specs, "nothing started")
}

func TestAProxyThatCannotBeBuiltFailsTheJob(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.Proxy = func(*runner.ClaimedJob) (runner.ProxyListener, error) {
		return nil, errors.New("no key")
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Could not prepare the egress proxy")
	assert.Empty(t, env.sandbox.specs)
}

func TestTheClaudeTokenNeverReachesTheOutcome(t *testing.T) {
	env := newEnv(t, writes("I used "+claudeToken+" to log in.", 0), newJob())
	env.runner.ClaudeToken = func() (string, error) { return claudeToken, nil }
	env.runner.Proxy = proxyOK

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.Contains(t, got.outcome, "the outcome contains a credential value")
	assert.NotContains(t, got.outcome, claudeToken)
}
