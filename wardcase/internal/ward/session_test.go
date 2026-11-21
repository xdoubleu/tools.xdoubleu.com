package ward_test

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

	"tools.xdoubleu.com/wardcase/internal/ward"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	id := []string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}
	all := append(slices.Clone(id), args...)
	out, err := exec.Command("git", all...).CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
}

// originRepo is a local repository on main, to clone from.
func originRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600))
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return dir
}

type sessionFixture struct {
	f      *fixture
	envs   [][]string
	origin string
}

func newSessionFixture(t *testing.T) *sessionFixture {
	t.Helper()
	sf := &sessionFixture{
		f: newFixture(t, ward.OverflowOff, false), envs: nil, origin: originRepo(t),
	}
	sf.f.r.Git = ward.HostGit
	sf.f.r.GitEnv = func(
		ctx context.Context, dir string, env []string, args ...string,
	) (string, error) {
		sf.envs = append(sf.envs, env)
		return ward.HostGitEnv(ctx, dir, env, args...)
	}
	return sf
}

func (sf *sessionFixture) request(branch string) ward.SessionRequest {
	return ward.SessionRequest{
		Source: ward.Source{URL: sf.origin, Base: "main", Branch: branch, Token: ""},
		Prompt: "fix it", Harness: ward.Claude, Fallback: "", Overflow: "",
		OpenCodeModel: "",
	}
}

func TestASessionClonesTheRepoAndRunsOnANewBranchOffTheBase(t *testing.T) {
	sf := newSessionFixture(t)
	var seen string
	sf.f.box.act = func(_ int, r run, _ io.Writer) int {
		seen = r.spec.CloneDir
		assert.FileExists(t, filepath.Join(r.spec.CloneDir, "a.txt"))
		head := gitIn(t, r.spec.CloneDir, "rev-parse", "--abbrev-ref", "HEAD")
		assert.Equal(t, "agent/one", head)
		return 0
	}
	res, err := sf.f.r.RunSession(context.Background(), sf.request("agent/one"))
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, ward.Claude, res.Harness)
	assert.False(t, res.Continued)
	assert.Equal(t, "agent/one", res.Branch)
	assert.Empty(t, res.Path, "nothing left, so the worktree is removed")
	assert.NoDirExists(t, seen)
	assert.Equal(t, "fix it", sf.f.box.runs[0].stdin)
}

func TestASessionReportsWhatItLeft(t *testing.T) {
	sf := newSessionFixture(t)
	sf.f.box.act = func(_ int, r run, _ io.Writer) int {
		write(t, r.spec.CloneDir, "new.txt")
		gitIn(t, r.spec.CloneDir, "add", "new.txt")
		gitIn(t, r.spec.CloneDir, "commit", "-q", "-m", "work")
		write(t, r.spec.CloneDir, "wip.txt")
		return 3
	}
	res, err := sf.f.r.RunSession(context.Background(), sf.request("agent/two"))
	require.NoError(t, err)
	assert.Equal(t, 3, res.ExitCode)
	assert.Equal(t, 1, res.Left.Unpushed)
	assert.Equal(t, 1, res.Left.Uncommitted)
	require.NotEmpty(t, res.Path)
	assert.DirExists(t, res.Path)
}

func TestASecondSessionReusesTheCloneAndSeesNewCommits(t *testing.T) {
	sf := newSessionFixture(t)
	_, err := sf.f.r.RunSession(context.Background(), sf.request("agent/a"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(sf.origin, "b.txt"), []byte("b"), 0o600))
	gitIn(t, sf.origin, "add", ".")
	gitIn(t, sf.origin, "commit", "-q", "-m", "second")

	sf.f.box.act = func(_ int, r run, _ io.Writer) int {
		assert.FileExists(t, filepath.Join(r.spec.CloneDir, "b.txt"))
		return 0
	}
	_, err = sf.f.r.RunSession(context.Background(), sf.request("agent/b"))
	require.NoError(t, err)
	caches, err := filepath.Glob(filepath.Join(sf.f.r.StateDir, "repos", "*", "repo"))
	require.NoError(t, err)
	assert.Len(t, caches, 1)
}

func TestABrokenCacheIsClonedAgain(t *testing.T) {
	sf := newSessionFixture(t)
	_, err := sf.f.r.RunSession(context.Background(), sf.request("agent/a"))
	require.NoError(t, err)
	caches, _ := filepath.Glob(filepath.Join(sf.f.r.StateDir, "repos", "*", "repo"))
	require.Len(t, caches, 1)
	require.NoError(t, os.RemoveAll(filepath.Join(caches[0], "objects")))

	_, err = sf.f.r.RunSession(context.Background(), sf.request("agent/b"))
	require.NoError(t, err)
}

func TestASessionNeedsTheBaseBranchToExist(t *testing.T) {
	sf := newSessionFixture(t)
	req := sf.request("agent/a")
	req.Base = "nope"
	_, err := sf.f.r.RunSession(context.Background(), req)
	require.ErrorContains(t, err, "no branch nope")
	assert.Empty(t, sf.f.box.runs)
}

func TestATokenReachesGitInTheEnvironmentNotTheArguments(t *testing.T) {
	sf := newSessionFixture(t)
	req := sf.request("agent/a")
	req.Token = "s3cret"
	_, err := sf.f.r.RunSession(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, sf.envs)
	joined := strings.Join(sf.envs[0], "\n")
	assert.Contains(t, joined, "http.extraHeader")
	assert.NotContains(t, joined, "s3cret", "only its base64 form")
	for _, run := range sf.f.box.runs {
		assert.NotContains(t, strings.Join(run.command, " "), "s3cret")
		for k, v := range run.spec.Env {
			assert.NotContains(t, k+v, "s3cret")
		}
	}

	sf.f.r.GitEnv = nil
	req.Branch = "agent/b"
	_, err = sf.f.r.RunSession(context.Background(), req)
	require.ErrorContains(t, err, "GitEnv")
}

func TestAUsageLimitWithAutoContinuesOnTheFallbackInTheSameWorktree(t *testing.T) {
	sf := newSessionFixture(t)
	sf.f.r.Secrets = memSecrets{ward.SecretAnthropic: "oat", ward.SecretOpenRouter: "k"}
	req := sf.request("agent/a")
	req.Overflow, req.Fallback, req.OpenCodeModel = ward.OverflowAuto, ward.OpenCode, "v/m"
	var dirs []string
	sf.f.box.act = func(i int, r run, out io.Writer) int {
		dirs = append(dirs, r.spec.CloneDir)
		if i == 0 {
			_, _ = io.WriteString(out,
				`{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}`+"\n")
			write(t, r.spec.CloneDir, "half.txt")
			return 1
		}
		return 0
	}
	res, err := sf.f.r.RunSession(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, sf.f.box.runs, 2)
	assert.Equal(t, dirs[0], dirs[1], "the same worktree")
	assert.Contains(t, sf.f.box.runs[1].stdin, "wardcase-handoff.md")
	assert.Equal(t, ward.OpenCode, res.Harness)
	assert.True(t, res.Continued)
	assert.False(t, res.Limited)
	assert.Equal(t, 1, res.Left.Uncommitted)
}

func TestAUsageLimitWithOverflowOffStops(t *testing.T) {
	sf := newSessionFixture(t)
	req := sf.request("agent/a")
	req.Fallback = ward.OpenCode
	sf.f.box.act = func(_ int, _ run, out io.Writer) int {
		_, _ = io.WriteString(out,
			`{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}`+"\n")
		return 1
	}
	res, err := sf.f.r.RunSession(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, sf.f.box.runs, 1)
	assert.True(t, res.Limited)
	assert.False(t, res.Continued)
}

func TestASessionRequestIsValidated(t *testing.T) {
	sf := newSessionFixture(t)
	cases := map[string]func(*ward.SessionRequest){
		"harness":  func(r *ward.SessionRequest) { r.Harness = "vim" },
		"prompt":   func(r *ward.SessionRequest) { r.Prompt = "" },
		"overflow": func(r *ward.SessionRequest) { r.Overflow = "ask" },
		"twice": func(r *ward.SessionRequest) {
			r.Overflow, r.Fallback = "auto", ward.Claude
		},
		"url":    func(r *ward.SessionRequest) { r.URL = "" },
		"branch": func(r *ward.SessionRequest) { r.Branch = "" },
	}
	for name, mutate := range cases {
		req := sf.request("agent/a")
		mutate(&req)
		res, err := sf.f.r.RunSession(context.Background(), req)
		require.Error(t, err, name)
		assert.Equal(t, -1, res.ExitCode, name)
	}
	assert.Empty(t, sf.f.box.runs)
}

func TestARunnerErrorStillReportsTheWorktree(t *testing.T) {
	sf := newSessionFixture(t)
	sf.f.box.openErr = io.ErrClosedPipe
	res, err := sf.f.r.RunSession(context.Background(), sf.request("agent/a"))
	require.ErrorIs(t, err, io.ErrClosedPipe)
	assert.Equal(t, -1, res.ExitCode)
	assert.Equal(t, "agent/a", res.Branch)
}
