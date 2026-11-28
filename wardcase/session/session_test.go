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
	cmds [][]string
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
	_ context.Context, command []string, _ string, out io.Writer,
) (int, error) {
	s.box.runs++
	s.box.cmds = append(s.box.cmds, command)
	return s.box.act(s.spec, out), nil
}

func (fakeSession) Close() {}

type secrets map[string]string

func (s secrets) Get(name string) (string, error) { return s[name], nil }

// realDir is a temp directory with symlinks resolved, as git reports it.
func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

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

	box := &fakeBox{runs: 0, cmds: nil, act: func(spec vm.SandboxSpec, _ io.Writer) int {
		git(t, spec.CloneDir, "commit", "-q", "--allow-empty", "-m", "work")
		return 0
	}}
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          box, Secrets: secrets{"anthropic": "oat"}, StateDir: t.TempDir(),
		TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	res, err := r.Run(context.Background(), session.Request{
		URL: origin, Base: "main", Branch: "agent/x", Token: "", Prompt: "do it",
		Harness: session.Claude, Overflow: session.OverflowOff, Fallback: "",
		OpenCodeModel: "", Dir: "", ResumeID: "", Extra: nil,
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
		Marketplaces: nil,
		Box:          &fakeBox{runs: 0, cmds: nil, act: nil}, Secrets: secrets{},
		StateDir: t.TempDir(), TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	_, err := r.Run(context.Background(), session.Request{
		URL: "x", Base: "main", Branch: "b", Token: "", Prompt: "",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
		Dir: "", ResumeID: "", Extra: nil,
	})
	require.Error(t, err)
}

func TestASessionRunsInAnExistingCheckoutOnItsBranchAndResumes(t *testing.T) {
	dir := realDir(t)
	git(t, dir, "init", "-q", "-b", "agent/x")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0o600))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "init")

	box := &fakeBox{runs: 0, cmds: nil, act: func(spec vm.SandboxSpec, out io.Writer) int {
		assert.Equal(t, dir, spec.CloneDir)
		_, _ = io.WriteString(out,
			`{"type":"system","subtype":"init","session_id":"s-1","model":"claude-x"}`+"\n")
		return 0
	}}
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          box, Secrets: secrets{"anthropic": "oat"}, StateDir: t.TempDir(),
		TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	res, err := r.Run(context.Background(), session.Request{
		Dir: dir, URL: "", Base: "", Branch: "agent/x", Token: "", Prompt: "fix",
		Harness: session.Claude, Overflow: session.OverflowOff, Fallback: "",
		OpenCodeModel: "", ResumeID: "s-0", Extra: nil,
	})
	require.NoError(t, err)
	assert.Equal(t, "s-1", res.SessionID)
	assert.Equal(t, "claude-x", res.Model)
	assert.Equal(t, "agent/x", res.Branch)
	assert.Equal(t, dir, res.Path)
	assert.DirExists(t, dir)
	assert.Equal(t, []string{"--resume", "s-0"}, box.cmds[0][len(box.cmds[0])-2:])
}

func TestAnExistingCheckoutOnAnotherBranchFails(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          &fakeBox{runs: 0, cmds: nil, act: nil}, Secrets: secrets{},
		StateDir: t.TempDir(), TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	_, err := r.Run(context.Background(), session.Request{
		Dir: dir, URL: "", Base: "", Branch: "agent/x", Token: "", Prompt: "p",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
		ResumeID: "", Extra: nil,
	})
	require.ErrorContains(t, err, "not agent/x")
}

func TestAnExistingCheckoutKeepsItsWorkAndOverflows(t *testing.T) {
	dir := realDir(t)
	git(t, dir, "init", "-q", "-b", "agent/x")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	limit := `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}` + "\n"
	box := &fakeBox{runs: 0, cmds: nil, act: nil}
	box.act = func(_ vm.SandboxSpec, out io.Writer) int {
		if box.runs == 1 {
			_, _ = io.WriteString(out, limit)
			return 1
		}
		git(t, dir, "commit", "-q", "--allow-empty", "-m", "more")
		return 0
	}
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          box, Secrets: secrets{"anthropic": "oat", "openrouter": "k"},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	res, err := r.Run(context.Background(), session.Request{
		Dir: dir, URL: "", Base: "", Branch: "", Token: "", Prompt: "p",
		Harness: session.Claude, Overflow: session.OverflowAuto,
		Fallback: session.OpenCode, OpenCodeModel: "vendor/model", ResumeID: "", Extra: nil,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, box.runs)
	assert.True(t, res.Continued)
	assert.Equal(t, session.OpenCode, res.Harness)
	assert.Equal(t, dir, res.Path)
	assert.DirExists(t, dir)
}

func TestAnExistingDirThatIsNotACheckoutOrIsDetachedFails(t *testing.T) {
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          &fakeBox{runs: 0, cmds: nil, act: nil}, Secrets: secrets{},
		StateDir: t.TempDir(), TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	req := session.Request{
		Dir: t.TempDir(), URL: "", Base: "", Branch: "", Token: "", Prompt: "p",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
		ResumeID: "", Extra: nil,
	}
	_, err := r.Run(context.Background(), req)
	require.ErrorContains(t, err, "not a git checkout")

	git(t, req.Dir, "init", "-q", "-b", "main")
	git(t, req.Dir, "commit", "-q", "--allow-empty", "-m", "init")
	git(t, req.Dir, "checkout", "-q", "--detach")
	_, err = r.Run(context.Background(), req)
	require.ErrorContains(t, err, "no branch checked out")
}

func TestAnExtraReachesTheVM(t *testing.T) {
	var spec vm.SandboxSpec
	box := &fakeBox{runs: 0, cmds: nil, act: func(s vm.SandboxSpec, _ io.Writer) int {
		spec = s
		return 0
	}}
	r := session.New(session.Options{
		Marketplaces: nil,
		Box:          box, Secrets: secrets{"anthropic": "oat"}, StateDir: t.TempDir(),
		TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	dir, out := t.TempDir(), t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	_, err := r.Run(context.Background(), session.Request{
		Dir: dir, URL: "", Base: "", Branch: "main", Token: "", Prompt: "p",
		Harness: session.Claude, Overflow: "", Fallback: "", OpenCodeModel: "",
		ResumeID: "",
		Extra: &session.Extra{
			Env: map[string]string{"FACTORY_JOB": "j"}, OutcomeDir: out,
			SessionDir: "", Board: nil, MCP: nil,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "j", spec.Env["FACTORY_JOB"])
	assert.Equal(t, out, spec.OutcomeDir)
}

func TestDeviceMarketplacesReadTheDeviceConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := session.DeviceMarketplaces()
	require.NoError(t, err)
	assert.Empty(t, got, "no config, no marketplaces")

	dir := filepath.Join(home, ".config", "wardcase")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	body := "[[marketplace]]\nname = \"skills\"\nurl = \"https://h/s\"\n" +
		"plugins = [\"dev\"]\n"
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(body), 0o600))
	got, err = session.DeviceMarketplaces()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "skills", got[0].Name)

	require.NoError(t, os.WriteFile(cfg, []byte("x ="), 0o600))
	_, err = session.DeviceMarketplaces()
	assert.Error(t, err)
}
