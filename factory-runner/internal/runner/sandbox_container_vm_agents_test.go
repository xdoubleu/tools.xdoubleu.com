//go:build container

package runner_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// seen is one request that reached the fake upstream.
type seen struct{ method, path, auth string }

// recordingUpstream answers 401 to everything, so an agent fails at once, and records
// what reached it.
type recordingUpstream struct {
	srv  *httptest.Server
	mu   sync.Mutex
	seen []seen
}

func newRecordingUpstream(t *testing.T) *recordingUpstream {
	t.Helper()
	u := &recordingUpstream{srv: nil, mu: sync.Mutex{}, seen: nil}
	handler := func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		u.seen = append(u.seen, seen{r.Method, r.URL.Path, r.Header.Get("Authorization")})
		u.mu.Unlock()
		http.Error(w, `{"error":"test upstream"}`, http.StatusUnauthorized)
	}
	u.srv = httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(u.srv.Close)
	return u
}

func (u *recordingUpstream) find(pathPart string) []seen {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []seen
	for _, s := range u.seen {
		if strings.Contains(s.path, pathPart) {
			out = append(out, s)
		}
	}
	return out
}

// agentRun runs one agent command in a real VM behind a real job proxy whose
// upstreams are the recording server.
func agentRun(
	t *testing.T,
	up *recordingUpstream,
	env map[string]string,
	command []string,
) string {
	t.Helper()
	proxy, err := runner.NewEgressProxy(runner.EgressConfig{
		Server:        up.srv.URL,
		MCPToken:      func(context.Context) (string, error) { return "board-token", nil },
		OpenRouterKey: func() (string, error) { return "real-openrouter-key", nil },
		MCPRejected:   nil, Transport: nil,
		Anthropic: up.srv.URL, OpenRouter: up.srv.URL + "/api", Log: nil,
	})
	require.NoError(t, err)

	box := &runner.ContainerSandbox{
		Image: "factory-job:dev", Exec: runner.OSExecer{BinDirs: nil}, Proxy: nil,
		ProxyPort: runner.EgressProxyPort, GoModCache: "", NPMCache: "",
		CPUs: "", Memory: "", TempDir: t.TempDir(),
	}
	dir := t.TempDir()
	dirs := []string{
		filepath.Join(dir, "clone"), filepath.Join(dir, "out"), filepath.Join(dir, "session"),
	}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(d, 0o777))
		require.NoError(t, os.Chmod(d, 0o777))
	}
	env["FACTORY_JOB"] = "realvm-agent"
	var log bytes.Buffer
	_, err = box.Run(context.Background(), runner.SandboxSpec{
		CloneDir: dirs[0], OutcomeDir: dirs[1], SessionDir: dirs[2], Env: env,
		SetupCommands: "", Command: append([]string{"timeout", "90"}, command...),
		Proxy: proxy, Stdin: "say ok\n", Log: &log,
	})
	require.NoError(t, err, log.String())
	return log.String()
}

func TestRealVMClaudeReachesItsModelAndTheBoardOnlyThroughTheProxy(t *testing.T) {
	up := newRecordingUpstream(t)
	env := runner.ClaudeCode{}.SessionEnv(runner.SandboxSessionDir)
	env["CLAUDE_CODE_OAUTH_TOKEN"] = "vm-setup-token-value"

	out := agentRun(t, up, env, runner.ClaudeCode{}.Command())

	model := up.find("/v1/messages")
	require.NotEmpty(t, model, "Claude's model call went through /anthropic: %s", out)
	assert.Equal(t, "Bearer vm-setup-token-value", model[0].auth,
		"the VM's own token passes through to Anthropic")

	board := up.find("/apps/mcp")
	require.NotEmpty(t, board, "Claude's MCP client reached the board through /mcp: %s", out)
	assert.Equal(t, "Bearer board-token", board[0].auth,
		"the host added the job account's token; the VM never held it")
}

func TestRealVMOpenCodeReachesOpenRouterAndTheBoardOnlyThroughTheProxy(t *testing.T) {
	up := newRecordingUpstream(t)
	h := runner.OpenCode{Model: "vendor/model"}
	env := h.SessionEnv(runner.SandboxSessionDir)

	out := agentRun(t, up, env, h.Command())

	model := up.find("/api/v1/")
	require.NotEmpty(t, model, "OpenCode's model call went through /openrouter: %s", out)
	assert.Equal(t, "Bearer real-openrouter-key", model[0].auth,
		"the host replaced the VM's placeholder key")

	board := up.find("/apps/mcp")
	if assert.NotEmpty(t, board, "OpenCode's MCP client reached the board: %s", out) {
		assert.Equal(t, "Bearer board-token", board[0].auth)
	}
}
