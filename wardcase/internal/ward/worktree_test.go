package ward_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

// repoGit plays a repository at top whose worktrees are directories under the
// state dir; status, ahead and own are what each worktree holds.
type repoGit struct {
	top      string
	status   string
	unpushed string
	own      string
	calls    []string
}

func (g *repoGit) git(_ context.Context, dir string, args ...string) (string, error) {
	cmd := strings.Join(args, " ")
	g.calls = append(g.calls, dir+": "+cmd)
	switch {
	case cmd == "rev-parse --show-toplevel":
		if strings.HasPrefix(dir, g.top) || strings.Contains(dir, "worktrees") {
			return dir + "\n", nil
		}
		return g.top + "\n", nil
	case strings.HasPrefix(cmd, "rev-parse --path-format=absolute"):
		return g.top + "/.git\n", nil
	case cmd == "rev-parse HEAD":
		return "base1\n", nil
	case cmd == "rev-parse --abbrev-ref HEAD":
		return "wardcase/x\n", nil
	case strings.HasPrefix(cmd, "worktree add"):
		return "", os.MkdirAll(args[len(args)-2], 0o700)
	case strings.HasPrefix(cmd, "worktree remove"):
		return "", os.RemoveAll(args[len(args)-1])
	case strings.HasPrefix(cmd, "config --get"):
		return "base1\n", nil
	case strings.HasPrefix(cmd, "config"), strings.HasPrefix(cmd, "branch -D"):
		return "", nil
	case cmd == "status --porcelain":
		return g.status, nil
	case cmd == "rev-list --count HEAD --not --remotes base1":
		return g.unpushed, nil
	case cmd == "rev-list --count HEAD --not base1":
		return g.own, nil
	}
	return "", errors.New("unexpected git " + cmd)
}

func worktreeFixture(t *testing.T) (*fixture, *repoGit) {
	t.Helper()
	f := newFixture(t, ward.OverflowOff, false)
	g := &repoGit{
		top: "/Users/me/repo", status: "", unpushed: "0\n", own: "0\n", calls: nil,
	}
	f.r.Git = g.git
	return f, g
}

func runIn(t *testing.T, f *fixture, args ...string) {
	t.Helper()
	_, err := f.r.Run(context.Background(), ward.Request{
		Dir: "/Users/me/repo", Harness: "claude", Args: args, Print: false, InPlace: false,
	})
	require.NoError(t, err)
}

func TestEachRunGetsItsOwnWorktreeOnANewBranch(t *testing.T) {
	f, g := worktreeFixture(t)
	runIn(t, f)
	runIn(t, f)
	a, b := f.box.runs[0], f.box.runs[1]
	assert.NotEqual(t, a.spec.CloneDir, b.spec.CloneDir)
	assert.NotEqual(t, a.spec.SessionDir, b.spec.SessionDir)
	for _, r := range []run{a, b} {
		assert.Contains(t, r.spec.CloneDir, f.r.StateDir)
		assert.Contains(t, r.spec.CloneDir, "/worktrees/")
	}
	assert.Contains(t, strings.Join(g.calls, "\n"), "worktree add -b wardcase/")
}

func TestInPlaceUsesTheCallersDirectory(t *testing.T) {
	f, g := worktreeFixture(t)
	_, err := f.r.Run(context.Background(), ward.Request{
		Dir: "/Users/me/repo", Harness: "claude", Args: nil, Print: false, InPlace: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "/Users/me/repo", f.box.runs[0].spec.CloneDir)
	assert.NotContains(t, strings.Join(g.calls, "\n"), "worktree")
	assert.NotContains(t, f.notes.String(), "left in")
}

func TestACleanUnchangedWorktreeIsRemoved(t *testing.T) {
	f, _ := worktreeFixture(t)
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "nothing left")
	_, err := os.Stat(f.box.runs[0].spec.CloneDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUncommittedAndUnpushedWorkIsKeptAndReported(t *testing.T) {
	f, g := worktreeFixture(t)
	g.status = " M a.go\n?? b.go\n"
	g.unpushed, g.own = "1\n", "1\n"
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "2 uncommitted changes")
	assert.Contains(t, f.notes.String(), "1 commit not on any remote")
	assert.Contains(t, f.notes.String(), "no pull request")
	_, err := os.Stat(f.box.runs[0].spec.CloneDir)
	require.NoError(t, err)
}

func TestPushedCommitsWithoutAMergedPRStayToo(t *testing.T) {
	f, g := worktreeFixture(t)
	g.own = "2\n"
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "left in")
	_, err := os.Stat(f.box.runs[0].spec.CloneDir)
	require.NoError(t, err)
}

func TestAMergedPullRequestLetsACleanWorktreeGo(t *testing.T) {
	f, g := worktreeFixture(t)
	g.own = "2\n"
	f.r.GH = func(context.Context, string, ...string) (string, error) {
		return `[{"state":"MERGED","url":"https://x/pr/1"}]`, nil
	}
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "removed")
}

func TestAnOpenPullRequestIsReported(t *testing.T) {
	f, g := worktreeFixture(t)
	g.own = "2\n"
	f.r.GH = func(context.Context, string, ...string) (string, error) {
		return `[{"state":"OPEN","url":"https://x/pr/1"}]`, nil
	}
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "pull request open https://x/pr/1")
}

func TestAFailingGHIsNoPullRequest(t *testing.T) {
	f, g := worktreeFixture(t)
	g.own = "2\n"
	f.r.GH = func(context.Context, string, ...string) (string, error) {
		return "", errors.New("gh missing")
	}
	runIn(t, f)
	assert.Contains(t, f.notes.String(), "no pull request")
}

func TestResumeReusesTheLatestWorktree(t *testing.T) {
	f, g := worktreeFixture(t)
	g.status = " M a.go\n"
	runIn(t, f)
	runIn(t, f, "--resume")
	first, second := f.box.runs[0].spec, f.box.runs[1].spec
	assert.Equal(t, first.CloneDir, second.CloneDir)
	assert.Equal(t, first.SessionDir, second.SessionDir)
}

func TestRunningInsideAWardcaseWorktreeReusesIt(t *testing.T) {
	f, g := worktreeFixture(t)
	g.status = " M a.go\n"
	runIn(t, f)
	wt := f.box.runs[0].spec.CloneDir
	_, err := f.r.Run(context.Background(), ward.Request{
		Dir: wt, Harness: "claude", Args: nil, Print: false, InPlace: false,
	})
	require.NoError(t, err)
	assert.Equal(t, wt, f.box.runs[1].spec.CloneDir)
	assert.Len(t, f.box.runs, 2)
}

func TestOverflowContinuesInTheSameWorktree(t *testing.T) {
	f, g := worktreeFixture(t)
	g.status = " M a.go\n"
	f.r.Config.Overflow = ward.OverflowAuto
	f.box.act = func(i int, r run, _ io.Writer) int {
		if i == 0 {
			writeTranscript(t, r, limitReply)
		}
		return 0
	}
	runIn(t, f)
	require.Len(t, f.box.runs, 2)
	assert.Equal(t, f.box.runs[0].spec.CloneDir, f.box.runs[1].spec.CloneDir)
}

func TestAFailedWorktreeStopsTheRun(t *testing.T) {
	f, _ := worktreeFixture(t)
	f.r.Git = func(_ context.Context, _ string, args ...string) (string, error) {
		if args[0] == "worktree" {
			return "", errors.New("boom")
		}
		return "/Users/me/repo\n", nil
	}
	_, err := f.r.Run(context.Background(), ward.Request{
		Dir: "/Users/me/repo", Harness: "claude", Args: nil, Print: false, InPlace: false,
	})
	require.ErrorContains(t, err, "create the worktree")
	assert.Empty(t, f.box.runs)
}
