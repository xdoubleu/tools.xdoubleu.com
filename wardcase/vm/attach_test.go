package vm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

func TestAnAttachedRunGetsATerminalAndTheCallersOwnInput(t *testing.T) {
	var stdin string
	exec := newExecScript(routesAnswer)
	exec.onExec = func(_ []string, in string) { stdin = in }
	box := sandboxWith(t, exec)
	session, err := box.Open(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	defer session.Close()

	attacher, ok := session.(vm.Attacher)
	require.True(t, ok, "the container session can attach")
	var out bytes.Buffer
	code, err := attacher.Attach(context.Background(), []string{"claude"},
		strings.NewReader("typed"), &out, true)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Equal(t, "typed", stdin)

	execs := exec.find("exec")
	require.Len(t, execs, 1)
	assert.Equal(t, []string{"container", "exec", "--interactive", "--tty", "--env-file"},
		execs[0][:5])
	assert.Equal(t, "claude", execs[0][len(execs[0])-1])
}

func TestAPlainExecHasNoTerminal(t *testing.T) {
	exec := newExecScript(routesAnswer)
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.NotContains(t, exec.find("exec")[0], "--tty")
}

func TestExtraMountsAreBoundAndTrustedByGit(t *testing.T) {
	exec := newExecScript(routesAnswer)
	s := spec(t, io.Discard)
	s.Mounts = []vm.Mount{
		{Source: "/Users/me/repo/.git", Target: "/Users/me/repo/.git", ReadOnly: false},
		{Source: "/opt/ref", Target: "/ref", ReadOnly: true},
	}
	_, err := sandboxWith(t, exec).Run(context.Background(), s)
	require.NoError(t, err)

	run := strings.Join(exec.find("run")[0], " ")
	assert.Contains(t, run,
		"--mount type=bind,source=/Users/me/repo/.git,target=/Users/me/repo/.git ")
	assert.Contains(t, run, "--mount type=bind,source=/opt/ref,target=/ref,readonly")
	script := strings.Join(exec.find("exec")[0], " ")
	assert.Contains(t, script, "safe.directory '/Users/me/repo/.git'")
	assert.Contains(t, script, "safe.directory '/ref'")
	assert.Contains(t, script, "BUNDLE_MIRROR__HTTPS://RUBYGEMS__ORG/")
	assert.Contains(t, script, `"$WARDCASE_PROXY" > "$HOME/.bundle/config"`)
}

func TestAMountTargetMustBeACleanAbsolutePath(t *testing.T) {
	for _, target := range []string{"relative", "/a/../b", "/a,b"} {
		exec := newExecScript(routesAnswer)
		s := spec(t, io.Discard)
		s.Mounts = []vm.Mount{{Source: "/src", Target: target, ReadOnly: false}}
		_, err := sandboxWith(t, exec).Run(context.Background(), s)
		require.Error(t, err, target)
		assert.Empty(t, exec.calls, "nothing starts for %q", target)
	}
}

func TestSkipInstallLeavesNodeModulesAlone(t *testing.T) {
	exec := newExecScript(routesAnswer)
	s := spec(t, io.Discard)
	s.SkipInstall = true
	_, err := sandboxWith(t, exec).Run(context.Background(), s)
	require.NoError(t, err)
	assert.NotContains(t, strings.Join(exec.find("exec")[0], " "), "npm ci")
}

func mcpOff() string { return vm.MCPEnv + "=" + vm.MCPOff }

func TestInteractiveClaudeGetsTheProxyAndTheCallersArguments(t *testing.T) {
	got := runHarness(t, "claude", vm.ClaudeCode{}.Interactive([]string{"--resume"}),
		claudeEnv(t, true), []string{"ANTHROPIC_BASE_URL"}, "factory-mcp.json")

	assert.Equal(t, proxyURL+"/anthropic", got.env["ANTHROPIC_BASE_URL"])
	assert.Equal(t, "--resume", got.args[0])
	assert.Equal(t, "--mcp-config", got.args[len(got.args)-2])
}

func TestClaudeWithMCPOffGetsNoMCPConfig(t *testing.T) {
	env := append(claudeEnv(t, true), mcpOff())
	got := runHarness(t, "claude", vm.ClaudeCode{}.Interactive(nil), env,
		[]string{"ANTHROPIC_BASE_URL"}, "factory-mcp.json")

	assert.Equal(t, proxyURL+"/anthropic", got.env["ANTHROPIC_BASE_URL"])
	assert.NotContains(t, got.args, "--mcp-config")
	assert.Empty(t, got.config)
}

func TestInteractiveOpenCodeDefaultsToTheModelWithoutMCP(t *testing.T) {
	h := vm.OpenCode{Model: "vendor/model"}
	got := runHarness(t, "opencode", h.Interactive([]string{"--prompt", "go on"}),
		append(openCodeEnv(true), mcpOff()), []string{"OPENCODE_CONFIG"},
		"opencode-job.json")

	want := filepath.Join(got.home, "opencode-job.json")
	assert.Equal(t, want, got.env["OPENCODE_CONFIG"])
	var cfg struct {
		Model string                     `json:"model"`
		MCP   map[string]json.RawMessage `json:"mcp"`
	}
	require.NoError(t, json.Unmarshal(got.config, &cfg), string(got.config))
	assert.Equal(t, "openrouter/vendor/model", cfg.Model)
	assert.Empty(t, cfg.MCP)
	assert.Equal(t, []string{"--standalone", "--prompt", "go on"}, got.args)
}

func TestLimitInTextReadsBothForms(t *testing.T) {
	hit, _ := vm.LimitInText("You’ve hit your session limit · resets 7am")
	assert.True(t, hit)
	hit, _ = vm.LimitInText("all good")
	assert.False(t, hit)
}

func TestLimitInErrorReadsProviderRefusalsAndClaudesText(t *testing.T) {
	for _, text := range []string{
		"Claude AI usage limit reached|0", "Insufficient credits", `{"code":429}`,
	} {
		hit, _ := vm.LimitInError(text)
		assert.True(t, hit, text)
	}
	hit, _ := vm.LimitInError("connection reset")
	assert.False(t, hit)
}
