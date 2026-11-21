package session_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/session"
	"tools.xdoubleu.com/wardcase/vm"
)

// fakeBox plays the harness with act.
type fakeBox struct {
	act  func(spec vm.SandboxSpec, out io.Writer) int
	runs int
}

func (b *fakeBox) Run(ctx context.Context, spec vm.SandboxSpec) (int, error) {
	s, _ := b.Open(ctx, spec)
	return s.Exec(ctx, spec.Command, spec.Stdin, spec.Log)
}

func (b *fakeBox) Open(
	_ context.Context, spec vm.SandboxSpec,
) (vm.SandboxSession, error) {
	return fakeSession{box: b, spec: spec}, nil
}

type fakeSession struct {
	box  *fakeBox
	spec vm.SandboxSpec
}

func (s fakeSession) Exec(
	_ context.Context, _ []string, _ string, out io.Writer,
) (int, error) {
	s.box.runs++
	return s.box.act(s.spec, out), nil
}

func (fakeSession) Close() {}

type secrets map[string]string

func (s secrets) Get(name string) (string, error) { return s[name], nil }

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	id := []string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}
	all := append(slices.Clone(id), args...)
	out, err := exec.Command("git", all...).CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestACallerWithNoCheckoutRunsASessionFromAURLAndBranch(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(origin, "a.txt"), nil, 0o600))
	git(t, origin, "add", ".")
	git(t, origin, "commit", "-q", "-m", "init")

	box := &fakeBox{runs: 0, act: func(spec vm.SandboxSpec, _ io.Writer) int {
		git(t, spec.CloneDir, "commit", "-q", "--allow-empty", "-m", "work")
		return 0
	}}
	r := session.New(session.Options{
		Box: box, Secrets: secrets{"anthropic": "oat"}, StateDir: t.TempDir(),
		TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	res, err := r.Run(context.Background(), session.Request{
		URL: origin, Base: "main", Branch: "agent/x", Token: "", Prompt: "do it",
		Harness: session.Claude, Overflow: session.OverflowOff, Fallback: "",
		OpenCodeModel: "",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, box.runs)
	assert.Equal(t, "agent/x", res.Branch)
	assert.Equal(t, 1, res.Unpushed)
	assert.True(t, res.Left())
	assert.DirExists(t, res.Path)
	assert.True(t, strings.Contains(res.Path, "worktrees"))
}

func TestARequestWithoutAPromptFails(t *testing.T) {
	r := session.New(session.Options{
		Box: &fakeBox{runs: 0, act: nil}, Secrets: secrets{}, StateDir: t.TempDir(),
		TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	_, err := r.Run(context.Background(), session.Request{
		URL: "x", Base: "main", Branch: "b", Token: "", Prompt: "",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
	})
	require.Error(t, err)
}
