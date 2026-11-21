//go:build container

package ward_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

// scriptBox runs a fixed script in the real VM instead of the harness.
type scriptBox struct {
	inner  vm.SessionSandbox
	script func(spec vm.SandboxSpec) string
}

func (b scriptBox) Run(ctx context.Context, spec vm.SandboxSpec) (int, error) {
	return b.inner.Run(ctx, spec)
}

func (b scriptBox) Open(ctx context.Context, spec vm.SandboxSpec) (vm.SandboxSession, error) {
	s, err := b.inner.Open(ctx, spec)
	if err != nil {
		return nil, err
	}
	return scriptSession{SandboxSession: s, script: b.script(spec)}, nil
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

func hostGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

// Needs a Mac with `container` and WARDCASE_IMAGE, like TestRealVM*.
func TestRealVMTwoSessionsOnOneRepoNeverSeeEachOthersEdits(t *testing.T) {
	image := os.Getenv("WARDCASE_IMAGE")
	if image == "" {
		t.Skip("set WARDCASE_IMAGE=factory-job:<tag>")
	}
	repo := t.TempDir()
	hostGitRun(t, repo, "init", "-q")
	hostGitRun(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q",
		"--allow-empty", "-m", "init")
	require.NoError(t, os.Chmod(repo, 0o777))

	inner := &vm.ContainerSandbox{
		Image: image, Exec: vm.OSExecer{BinDirs: nil}, Proxy: nil,
		ProxyPort: vm.EgressProxyPort, GoModCache: "", NPMCache: "", CPUs: "",
		Memory: "", TempDir: t.TempDir(),
	}
	require.NoError(t, inner.Check(context.Background()))
	state := t.TempDir()

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]string{}
	for _, name := range []string{"one", "two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var notes bytes.Buffer
			r := &ward.Runner{
				Box: scriptBox{inner: inner, script: func(vm.SandboxSpec) string {
					return "echo " + name + " > /work/mine; sleep 5; ls /work"
				}},
				Secrets:  memSecrets{ward.SecretAnthropic: "oat-token"},
				Config:   ward.Config{Order: []string{"claude"}, Overflow: ward.OverflowOff},
				StateDir: state, TempDir: t.TempDir(),
				Git: func(ctx context.Context, dir string, args ...string) (string, error) {
					out, err := exec.CommandContext(ctx, "git",
						append([]string{"-C", dir}, args...)...).Output()
					return string(out), err
				},
				NewProxy: func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
					return vm.NewEgressProxy(cfg)
				},
				In: strings.NewReader(""), Out: io.Discard, Notes: &notes,
			}
			_, err := r.Run(context.Background(), ward.Request{
				Dir: repo, Harness: "claude", Args: []string{"x"}, Print: true,
			})
			assert.NoError(t, err)
			mu.Lock()
			seen[name] = notes.String()
			mu.Unlock()
		}()
	}
	wg.Wait()

	assert.NoFileExists(t, filepath.Join(repo, "mine"), "the caller's checkout is untouched")
	for name, notes := range seen {
		assert.Contains(t, notes, "left in", name)
	}
	trees, err := filepath.Glob(filepath.Join(state, "*", "worktrees", "*", "mine"))
	require.NoError(t, err)
	require.Len(t, trees, 2)
	a, _ := os.ReadFile(trees[0])
	b, _ := os.ReadFile(trees[1])
	assert.NotEqual(t, string(a), string(b))
}
