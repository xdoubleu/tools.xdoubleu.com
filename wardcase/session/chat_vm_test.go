//go:build container

package session_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/session"
	"tools.xdoubleu.com/wardcase/vm"
)

// Needs a Mac with `container`, the wardcase image in WARDCASE_IMAGE, and an
// OpenRouter key in WARDCASE_OPENROUTER_KEY; it spends a few tokens on a real model.
func TestRealVMChatRunsOpenCodeOverACP(t *testing.T) {
	image, key := os.Getenv("WARDCASE_IMAGE"), os.Getenv("WARDCASE_OPENROUTER_KEY")
	if image == "" || key == "" {
		t.Skip("set WARDCASE_IMAGE=<image> and WARDCASE_OPENROUTER_KEY")
	}
	model := os.Getenv("WARDCASE_OPENCODE_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	work := t.TempDir()
	run := func(dir string, args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t",
			"-c", "user.email=t@t"}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run(work, "init", "-q", "-b", "main")
	run(work, "commit", "-q", "--allow-empty", "-m", "init")
	bare := t.TempDir()
	run(bare, "clone", "-q", "--bare", work, bare+"/r.git")

	box := &vm.ContainerSandbox{
		Image: image, Exec: vm.OSExecer{BinDirs: nil}, Proxy: nil,
		ProxyPort: vm.EgressProxyPort, GoModCache: "", NPMCache: "", CPUs: "",
		Memory: "", TempDir: t.TempDir(),
	}
	require.NoError(t, box.Check(context.Background()))
	state := t.TempDir()
	require.NoError(t, os.Chmod(state, 0o777))

	r := session.New(session.Options{
		Box: box, Secrets: secrets{"openrouter": key}, StateDir: state,
		TempDir: t.TempDir(), Out: io.Discard, Notes: io.Discard, GH: nil,
	})
	var mu sync.Mutex
	var text strings.Builder
	var permissions int
	ctx := context.Background()
	chat, err := r.Start(ctx, session.ChatRequest{
		URL: bare + "/r.git", Base: "main", Branch: "agent/acp", Token: "",
		Harness: session.OpenCode, OpenCodeModel: model,
		OnEvent: func(e acp.Event) {
			mu.Lock()
			defer mu.Unlock()
			if e.Kind == "permission" {
				permissions++
			}
			text.WriteString(e.Text())
		},
	})
	require.NoError(t, err)

	reason, err := chat.Prompt(ctx,
		"Create a file named hello.txt containing the word hi, using your shell tool.")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	mu.Lock()
	assert.NotEmpty(t, text.String(), "the agent's reply streamed back")
	t.Logf("permissions allowed: %d", permissions)
	mu.Unlock()

	res := chat.Close(ctx)
	assert.Equal(t, "agent/acp", res.Branch)
	assert.True(t, res.Left(), "the file the session made is left")
	assert.FileExists(t, res.Path+"/hello.txt")
}
