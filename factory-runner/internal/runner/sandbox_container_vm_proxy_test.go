//go:build container

package runner_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// A real VM reaching a real proxy on its network's gateway: the only way out, with
// the real credential added on the way. `go test -tags container`, on a Mac.
func TestRealVMReachesOnlyTheProxyAndTheProxyAddsTheKey(t *testing.T) {
	var seenAuth, seenPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth, seenPath = r.Header.Get("Authorization"), r.URL.Path
		_, _ = io.WriteString(w, "from-upstream")
	}))
	defer up.Close()
	proxy, err := runner.NewEgressProxy(runner.EgressConfig{
		Server: "", MCPToken: nil, Transport: nil, Anthropic: up.URL,
		OpenRouterKey: func() (string, error) { return "real-key", nil },
		OpenRouter:    up.URL + "/api", Log: nil,
	})
	require.NoError(t, err)

	box := &runner.ContainerSandbox{
		Image: "factory-job:dev", Exec: runner.OSExecer{BinDirs: nil}, GoModCache: "", NPMCache: "",
		CPUs: "", Memory: "", TempDir: t.TempDir(), ProxyPort: runner.EgressProxyPort,
		Proxy: proxy,
	}
	dir := t.TempDir()
	dirs := []string{
		filepath.Join(dir, "clone"), filepath.Join(dir, "out"), filepath.Join(dir, "session"),
	}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(d, 0o777))
		require.NoError(t, os.Chmod(d, 0o777))
	}
	script := `
curl -sS -m 8 -X POST -H 'Authorization: Bearer replaced-by-the-host-proxy' \
  -d '{}' "$FACTORY_PROXY/openrouter/v1/chat/completions"; echo
curl -sS -m 8 -o /dev/null -w 'other=%{http_code}\n' "$FACTORY_PROXY/other"
curl -sS -m 4 -o /dev/null https://example.com && echo NET=leak || echo NET=blocked
curl -sS -m 4 -o /dev/null http://1.1.1.1 && echo NET=leak || echo NET=blocked
`
	var log bytes.Buffer
	code, err := box.Run(context.Background(), runner.SandboxSpec{
		CloneDir: dirs[0], OutcomeDir: dirs[1], SessionDir: dirs[2],
		Env:           map[string]string{"FACTORY_JOB": "realvm-proxy"},
		SetupCommands: "", Command: []string{"sh", "-c", script}, Stdin: "", Log: &log,
	})
	require.NoError(t, err, log.String())
	assert.Zero(t, code, log.String())

	out := log.String()
	assert.Contains(t, out, "from-upstream")
	assert.Contains(t, out, "other=403")
	assert.NotContains(t, out, "NET=leak")
	assert.Equal(t, "Bearer real-key", seenAuth, "the key was added on the host")
	assert.Equal(t, "/api/v1/chat/completions", seenPath)
}
