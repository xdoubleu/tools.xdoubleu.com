//go:build container

package vm_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

// The checkout wardcase mounts is the user's own: a git worktree whose shared git
// directory lies outside it, with the host user's ownership and modes. Runs a real
// VM: `WARDCASE_IMAGE=factory-job:<tag> go test -tags container -run
// TestRealVMOnAWorktree ./vm`.
func TestRealVMOnAWorktree(t *testing.T) {
	image := os.Getenv("WARDCASE_IMAGE")
	require.NotEmpty(t, image, "set WARDCASE_IMAGE")
	box := &vm.ContainerSandbox{
		Image: image, Exec: vm.OSExecer{BinDirs: nil}, Proxy: nil, ProxyPort: 0,
		GoModCache: "", NPMCache: "", CPUs: "", Memory: "", TempDir: t.TempDir(),
	}
	require.NoError(t, box.Check(context.Background()))

	// git records the resolved path (/private/var), so the mount must use it too.
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	repo, wt := filepath.Join(root, "repo"), filepath.Join(root, "wt")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q",
			"--allow-empty", "-m", "init"},
		{"-C", repo, "worktree", "add", "-q", wt},
	} {
		out, gitErr := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, gitErr, string(out))
	}
	outcome, session := filepath.Join(root, "out"), filepath.Join(root, "session")
	require.NoError(t, os.MkdirAll(outcome, 0o700))
	require.NoError(t, os.MkdirAll(session, 0o700))
	common := filepath.Join(repo, ".git")

	s, err := box.Open(context.Background(), vm.SandboxSpec{
		CloneDir: wt, OutcomeDir: outcome, SessionDir: session,
		Env:     map[string]string{"FACTORY_JOB": "wardcase-realvm"},
		Command: nil, Proxy: nil, Stdin: "", Log: nil,
		Mounts:      []vm.Mount{{Source: common, Target: common, ReadOnly: false}},
		SkipInstall: true,
	})
	require.NoError(t, err)
	defer s.Close()

	script := `echo hi > /work/made-in-vm && echo WRITE=ok
echo s > /session/s && echo SESSION=ok
git -c user.name=v -c user.email=v@v commit -q --allow-empty -m from-vm && echo GIT=ok`
	attacher, ok := s.(vm.Attacher)
	require.True(t, ok)
	var out bytes.Buffer
	code, err := attacher.Attach(context.Background(),
		[]string{"sh", "-c", script}, strings.NewReader(""), &out, false)
	require.NoError(t, err)
	t.Log(out.String())
	assert.Equal(t, 0, code, out.String())
	assert.Contains(t, out.String(), "WRITE=ok")
	assert.Contains(t, out.String(), "SESSION=ok")
	assert.Contains(t, out.String(), "GIT=ok")

	log, err := exec.Command("git", "-C", wt, "log", "-1", "--format=%s").Output()
	require.NoError(t, err)
	assert.Equal(t, "from-vm", strings.TrimSpace(string(log)), "the host sees the commit")
	info, err := os.Stat(filepath.Join(wt, "made-in-vm"))
	require.NoError(t, err)
	t.Logf("file made in the VM: mode %v", info.Mode())
}

// Every harness in the image starts on a terminal command line behind the proxy with MCP off.
func TestRealVMStartsEachHarnessInteractively(t *testing.T) {
	image := os.Getenv("WARDCASE_IMAGE")
	require.NotEmpty(t, image, "set WARDCASE_IMAGE")
	proxy, err := vm.NewEgressProxy(vm.EgressConfig{
		Server: "", MCPToken: nil, OpenRouterKey: nil, OpenAIKey: nil, MCPServers: nil,
		MCPRejected: nil, Transport: nil, Anthropic: "", OpenRouter: "", OpenAI: "",
		GoProxy: "", NPM: "", RubyGems: "", NixCache: "", NixSrc: "", Log: nil,
	})
	require.NoError(t, err)
	box := &vm.ContainerSandbox{
		Image: image, Exec: vm.OSExecer{BinDirs: nil}, Proxy: proxy,
		ProxyPort: vm.EgressProxyPort, GoModCache: "", NPMCache: "", CPUs: "", Memory: "",
		TempDir: t.TempDir(),
	}
	for _, h := range []interface {
		vm.Harness
		Interactive([]string) []string
	}{vm.ClaudeCode{}, vm.OpenCode{Model: "vendor/model"}, vm.Codex{}} {
		dir := t.TempDir()
		env := h.SessionEnv(vm.SandboxSessionDir)
		env["FACTORY_JOB"] = "wardcase-harness"
		env[vm.MCPEnv] = vm.MCPOff
		s, openErr := box.Open(context.Background(), vm.SandboxSpec{
			CloneDir: dir, OutcomeDir: t.TempDir(), SessionDir: t.TempDir(), Env: env,
			Command: nil, Proxy: nil, Stdin: "", Log: nil, Mounts: nil, SkipInstall: true,
		})
		require.NoError(t, openErr)
		attacher, ok := s.(vm.Attacher)
		require.True(t, ok)
		var out bytes.Buffer
		code, attachErr := attacher.Attach(context.Background(),
			h.Interactive([]string{"--version"}), strings.NewReader(""), &out, false)
		s.Close()
		require.NoError(t, attachErr)
		t.Logf("%T: %s", h, strings.TrimSpace(out.String()))
		assert.Equal(t, 0, code, out.String())
	}
}
