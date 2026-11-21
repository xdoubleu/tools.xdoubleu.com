//go:build container

package session_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/session"
	"tools.xdoubleu.com/wardcase/vm"
)

// scriptBox runs a fixed script in the real VM instead of the harness.
type scriptBox struct {
	inner  vm.SessionSandbox
	script string
}

func (b scriptBox) Run(ctx context.Context, spec vm.SandboxSpec) (int, error) {
	return b.inner.Run(ctx, spec)
}

func (b scriptBox) Open(ctx context.Context, spec vm.SandboxSpec) (vm.SandboxSession, error) {
	s, err := b.inner.Open(ctx, spec)
	if err != nil {
		return nil, err
	}
	return scriptSession{SandboxSession: s, script: b.script}, nil
}

type scriptSession struct {
	vm.SandboxSession
	script string
}

func (s scriptSession) Exec(
	ctx context.Context, _ []string, _ string, log io.Writer,
) (int, error) {
	return s.SandboxSession.Exec(ctx, []string{"sh", "-c", s.script}, "", log)
}

// Needs a Mac with `container` and WARDCASE_IMAGE, like TestRealVM*.
func TestRealVMSessionFromALocalBareRepo(t *testing.T) {
	image := os.Getenv("WARDCASE_IMAGE")
	if image == "" {
		t.Skip("set WARDCASE_IMAGE=factory-job:<tag>")
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

	inner := &vm.ContainerSandbox{
		Image: image, Exec: vm.OSExecer{BinDirs: nil}, Proxy: nil,
		ProxyPort: vm.EgressProxyPort, GoModCache: "", NPMCache: "", CPUs: "",
		Memory: "", TempDir: t.TempDir(),
	}
	require.NoError(t, inner.Check(context.Background()))
	state := t.TempDir()
	require.NoError(t, os.Chmod(state, 0o777))

	r := session.New(session.Options{
		Box:     scriptBox{inner: inner, script: "echo hi > /work/made"},
		Secrets: secrets{"anthropic": "oat"}, StateDir: state, TempDir: t.TempDir(),
		Out: io.Discard, Notes: io.Discard, GH: nil,
	})
	res, err := r.Run(context.Background(), session.Request{
		URL: bare + "/r.git", Base: "main", Branch: "agent/vm", Token: "", Prompt: "x",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, 1, res.Uncommitted, "the file the VM made")
	assert.FileExists(t, res.Path+"/made")
}
