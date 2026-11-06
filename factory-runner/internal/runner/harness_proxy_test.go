package runner_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const proxyURL = "http://10.231.7.1:8118"

// observed is what a fake agent binary saw when the harness's start step ran it.
type observed struct {
	args   []string
	env    map[string]string
	config []byte
	home   string
}

// runHarness runs argv with a fake binary called name first on PATH. The fake writes
// its arguments, the named environment variables, and the file the harness wrote
// under $HOME, so the start step is checked as the sandbox would run it.
func runHarness(
	t *testing.T,
	name string,
	argv []string,
	extraEnv []string,
	vars []string,
	configFile string,
) observed {
	t.Helper()
	bin, home := t.TempDir(), t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do printf 'ARG:%s\\n' \"$a\"; done\n"
	for _, v := range vars {
		script += "printf 'ENV:" + v + "=%s\\n' \"${" + v + ":-}\"\n"
	}
	script += "[ -f \"$HOME/" + configFile + "\" ] && cat \"$HOME/" + configFile + "\"\n"
	script += "exit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755))

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), append([]string{
		"PATH=" + bin + ":" + os.Getenv("PATH"), "HOME=" + home,
	}, extraEnv...)...)
	cmd.Stdin = strings.NewReader("the prompt\n")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	got := observed{args: nil, env: map[string]string{}, config: nil, home: home}
	var config []string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "ARG:"):
			got.args = append(got.args, strings.TrimPrefix(line, "ARG:"))
		case strings.HasPrefix(line, "ENV:"):
			k, v, _ := strings.Cut(strings.TrimPrefix(line, "ENV:"), "=")
			got.env[k] = v
		case line != "":
			config = append(config, line)
		}
	}
	got.config = []byte(strings.Join(config, "\n"))
	return got
}

func claudeEnv(t *testing.T, withProxy bool) []string {
	t.Helper()
	seed := t.TempDir()
	settings := filepath.Join(seed, "settings.json")
	require.NoError(t, os.WriteFile(settings, []byte("{}"), 0o600))
	env := []string{"CLAUDE_CONFIG_DIR=" + t.TempDir(), "CLAUDE_CONFIG_SEED=" + seed}
	if withProxy {
		env = append(env, "FACTORY_PROXY="+proxyURL)
	}
	return env
}

func TestClaudeIsPointedAtTheProxyAndTheBoardThroughIt(t *testing.T) {
	got := runHarness(t, "claude", runner.ClaudeCode{}.Command(), claudeEnv(t, true),
		[]string{"ANTHROPIC_BASE_URL"}, "factory-mcp.json")

	assert.Equal(t, proxyURL+"/anthropic", got.env["ANTHROPIC_BASE_URL"])
	require.GreaterOrEqual(t, len(got.args), 2)
	assert.Equal(t, []string{"--mcp-config", filepath.Join(got.home, "factory-mcp.json")},
		got.args[len(got.args)-2:])
	assert.Equal(t, "-p", got.args[0])

	type server struct {
		Type    string            `json:"type"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	var mcp struct {
		Servers map[string]server `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal(got.config, &mcp), string(got.config))
	assert.Equal(t, "http", mcp.Servers["board"].Type)
	assert.Equal(t, proxyURL+"/mcp", mcp.Servers["board"].URL)
	assert.Empty(t, mcp.Servers["board"].Headers, "the host adds the credential")
}

func TestAResumedClaudeJobKeepsItsResumeFlagAndGetsTheSameConfig(t *testing.T) {
	got := runHarness(t, "claude", runner.ClaudeCode{}.Resume("abc"), claudeEnv(t, true),
		nil, "factory-mcp.json")

	assert.Contains(t, strings.Join(got.args, " "), "--resume abc")
	assert.Equal(t, "--mcp-config", got.args[len(got.args)-2])
}

func TestWithoutAProxyClaudeIsLeftAlone(t *testing.T) {
	got := runHarness(t, "claude", runner.ClaudeCode{}.Command(), claudeEnv(t, false),
		[]string{"ANTHROPIC_BASE_URL"}, "factory-mcp.json")

	assert.Empty(t, got.env["ANTHROPIC_BASE_URL"])
	assert.NotContains(t, got.args, "--mcp-config")
	assert.Empty(t, got.config)
}

func openCodeEnv(withProxy bool) []string {
	if withProxy {
		return []string{"FACTORY_PROXY=" + proxyURL}
	}
	return nil
}

func TestOpenCodeIsPointedAtOpenRouterAndTheBoardThroughTheProxy(t *testing.T) {
	h := runner.OpenCode{Model: "vendor/model:free"}
	got := runHarness(t, "opencode", h.Command(), openCodeEnv(true),
		[]string{"OPENCODE_CONFIG"}, "opencode-job.json")

	want := filepath.Join(got.home, "opencode-job.json")
	assert.Equal(t, want, got.env["OPENCODE_CONFIG"])
	type provider struct {
		Options struct {
			BaseURL string `json:"baseURL"`
		} `json:"options"`
		Models map[string]json.RawMessage `json:"models"`
	}
	type remote struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	var cfg struct {
		Provider map[string]provider `json:"provider"`
		MCP      map[string]remote   `json:"mcp"`
	}
	require.NoError(t, json.Unmarshal(got.config, &cfg), string(got.config))
	assert.Equal(t, proxyURL+"/openrouter/v1", cfg.Provider["openrouter"].Options.BaseURL)
	assert.Contains(t, cfg.Provider["openrouter"].Models, "vendor/model:free")
	assert.Equal(t, "remote", cfg.MCP["board"].Type)
	assert.Equal(t, proxyURL+"/mcp", cfg.MCP["board"].URL)
	assert.Contains(t, strings.Join(got.args, " "), "-m openrouter/vendor/model:free")
}

func TestAResumedOpenCodeJobGetsTheSameConfig(t *testing.T) {
	h := runner.OpenCode{Model: "a/b"}
	got := runHarness(t, "opencode", h.Resume("ses_1"), openCodeEnv(true),
		[]string{"OPENCODE_CONFIG"}, "opencode-job.json")

	assert.NotEmpty(t, got.env["OPENCODE_CONFIG"])
	assert.Contains(t, strings.Join(got.args, " "), "--session ses_1")
}

func TestWithoutAProxyOpenCodeIsLeftAlone(t *testing.T) {
	h := runner.OpenCode{Model: "a/b"}
	got := runHarness(t, "opencode", h.Command(), openCodeEnv(false),
		[]string{"OPENCODE_CONFIG"}, "opencode-job.json")

	assert.Empty(t, got.env["OPENCODE_CONFIG"])
	assert.Empty(t, got.config)
}
