package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestAJobGetsTheDeviceMCPServersAndTheirNames(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	var given []runner.MCPServer
	env.runner.MCP = func() ([]runner.MCPServer, []string, error) {
		return []runner.MCPServer{{Name: "posthog", URL: "", Token: nil, Rejected: nil},
			{Name: "linear", URL: "", Token: nil, Rejected: nil}}, nil, nil
	}
	env.runner.Proxy = func(
		j *runner.ClaimedJob, servers []runner.MCPServer,
	) (runner.ProxyListener, error) {
		given = servers
		return proxyOK(j, nil)
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, given, 2)
	assert.Equal(t, "linear posthog", env.sandbox.specs[0].Env[runner.MCPServersEnv])
	assert.NotContains(t, env.api.done()[0].outcome, "not signed in")
}

func TestAJobWithoutDeviceMCPServersNamesNone(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.Proxy = proxyOK

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, env.sandbox.specs[0].Env, runner.MCPServersEnv)
}

func TestAServerThatIsNotSignedInIsNamedInTheOutcomeAndTheJobRuns(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.MCP = func() ([]runner.MCPServer, []string, error) {
		return nil, []string{"posthog"}, nil
	}
	env.runner.Proxy = proxyOK

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Contains(t, got.outcome, "not signed in")
	assert.Contains(t, got.outcome, "posthog")
	assert.NotContains(t, env.sandbox.specs[0].Env, runner.MCPServersEnv)
}

func TestAnUnreadableDeviceConfigIsReportedAndTheJobRuns(t *testing.T) {
	env := newEnv(t, writes("Done.", 0), newJob())
	env.runner.MCP = func() ([]runner.MCPServer, []string, error) {
		return nil, nil, errors.New("config.toml: unknown setting")
	}
	env.runner.Proxy = proxyOK

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Contains(t, got.outcome, "MCP servers could not be read")
}
