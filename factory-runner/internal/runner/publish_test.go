package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// fakeGitHub opens "pull requests" in memory.
type fakeGitHub struct {
	mu     sync.Mutex
	opened []runner.PullRequest
	tokens []string
	url    string
	err    error
}

func (f *fakeGitHub) OpenPullRequest(
	_ context.Context, token string, pr runner.PullRequest,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, pr)
	f.tokens = append(f.tokens, token)
	return f.url, f.err
}

const pushToken = "ghs_pushtoken_123456"

// publishing is a job env whose runner pushes to its local bare remote and opens
// pull requests on a fake GitHub; the push's argv and environment are recorded.
type publishing struct {
	*jobEnv
	gh   *fakeGitHub
	logs *bytes.Buffer
	mu   sync.Mutex
	args [][]string
	envs [][]string
}

func newPublishing(
	t *testing.T,
	sandbox func(context.Context, runner.SandboxSpec) (int, error),
) *publishing {
	t.Helper()
	env := newEnv(t, sandbox, newJob())
	p := &publishing{
		jobEnv: env, logs: &bytes.Buffer{}, mu: sync.Mutex{}, args: nil, envs: nil,
		gh: &fakeGitHub{
			mu: sync.Mutex{}, opened: nil, tokens: nil, err: nil,
			url: "https://github.com/xdoubleu/tools.xdoubleu.com/pull/7",
		},
	}
	env.api.pushToken = pushToken
	env.runner.GitHub = p.gh
	env.runner.ServerURL = "https://tools.example.com/"
	env.runner.Log = slog.New(slog.NewTextHandler(p.logs, nil))
	env.runner.Git.RunWithEnv = func(
		ctx context.Context, dir string, extra []string, args ...string,
	) ([]byte, error) {
		p.mu.Lock()
		p.args = append(p.args, args)
		p.envs = append(p.envs, extra)
		p.mu.Unlock()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), extra...)
		return cmd.CombinedOutput()
	}
	return p
}

func (p *publishing) remoteBranch(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", p.remote, "rev-parse", "--verify", "-q",
		"refs/heads/"+branch).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestAJobThatPassesTheChecksIsPushedAndGetsItsPullRequest(t *testing.T) {
	p := newPublishing(t, writes("Fix the login redirect\n\nChanged the handler.", 0))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Equal(t, "https://github.com/xdoubleu/tools.xdoubleu.com/pull/7", got.prURL)
	assert.NotEmpty(t, p.remoteBranch(t), "the branch reached the remote")

	require.Len(t, p.gh.opened, 1)
	pr := p.gh.opened[0]
	assert.Equal(t, "xdoubleu", pr.Owner)
	assert.Equal(t, "tools.xdoubleu.com", pr.Repo)
	assert.Equal(t, branch, pr.Head)
	assert.Equal(t, "main", pr.Base)
	assert.Equal(t, "Fix the login redirect", pr.Title)
	assert.Contains(t, pr.Body, "Changed the handler.")
	assert.Contains(t, pr.Body, "task `t`")
	assert.Contains(t, pr.Body, "https://tools.example.com/factory")
	assert.Equal(t, []string{pushToken}, p.gh.tokens)

	// The review copy is gone.
	assert.NoDirExists(t, filepath.Join(p.dir, "reviews", jobID+".git"))
}

func TestThePushTokenIsInTheEnvironmentAndNeverInArgv(t *testing.T) {
	p := newPublishing(t, writes("done", 0))
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.args, 1, "one push")
	for _, a := range p.args[0] {
		assert.NotContains(t, a, pushToken)
	}
	assert.Equal(t, "push", p.args[0][4])
	assert.NotContains(t, p.args[0], "--force")
	assert.NotContains(t, p.args[0], "-f")
	assert.Contains(t, p.args[0], "--no-verify")
	assert.Contains(t, p.args[0], "credential.helper=")

	joined := strings.Join(p.envs[0], "\n")
	assert.Contains(t, joined, "GIT_CONFIG_KEY_0=http.extraHeader")
	assert.Contains(t, joined, "GIT_TERMINAL_PROMPT=0")
	assert.Equal(t, strings.Join(runner.PushEnvForTest(pushToken), "\n"), joined)
	assert.NotContains(t, p.logs.String(), pushToken)
}

func TestWithoutAGitHubClientNothingIsAskedOrPushed(t *testing.T) {
	env := newEnv(t, writes("done", 0), newJob())
	env.api.pushToken = pushToken
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Zero(t, env.api.tokenAsks)
	got := env.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Empty(t, got.prURL)
}

func TestAJobThatFailsTheChecksAsksForNoToken(t *testing.T) {
	p := newPublishing(t, agentDoes("done", func(string) {}))
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Zero(t, p.api.tokenAsks)
	assert.Empty(t, p.gh.opened)
	assert.False(t, p.api.done()[0].succeeded)
}

func TestAServerThatCannotPushLeavesTheWorkOnTheMac(t *testing.T) {
	cases := map[string]struct {
		err   error
		token string
		want  string
	}{
		"not set up": {
			err:   &runner.Error{Code: "failed_precondition", Message: "x"},
			token: "", want: "pushing is not set up",
		},
		"a failure": {
			err: errors.New("boom"), token: "", want: "could not get a push token",
		},
		"no token": {err: nil, token: "", want: "could not get a push token"},
	}
	for name, tc := range cases {
		p := newPublishing(t, writes("Did it.", 0))
		p.api.pushToken, p.api.pushErr = tc.token, tc.err

		_, err := p.runner.Next(context.Background())
		require.NoError(t, err, name)

		got := p.api.done()[0]
		assert.True(t, got.succeeded, name+": the agent's work is fine")
		assert.Empty(t, got.prURL, name)
		assert.Contains(t, got.outcome, "Not pushed: "+tc.want, name)
		assert.Contains(t, got.outcome, "Did it.", name)
		assert.Empty(t, p.remoteBranch(t), name)
		assert.Empty(t, p.gh.opened, name)
	}
}

func TestABranchThatAlreadyHoldsOtherWorkOnGitHubIsNeverOverwritten(t *testing.T) {
	p := newPublishing(t, writes("Did it.", 0))
	// Someone (an earlier job) already put different work on the branch.
	other := filepath.Join(t.TempDir(), "other")
	require.NoError(t, os.MkdirAll(other, 0o755))
	gitIn(t, other, "clone", p.remote, ".")
	gitIn(t, other, "checkout", "-b", branch)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(other, "theirs.txt"), []byte("x"), 0o644),
	)
	gitIn(t, other, "add", ".")
	gitIn(t, other, "commit", "-m", "theirs")
	gitIn(t, other, "push", "origin", branch)
	before := p.remoteBranch(t)
	require.NotEmpty(t, before)

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Empty(t, got.prURL)
	assert.Contains(t, got.outcome, "never overwritten")
	assert.Equal(t, before, p.remoteBranch(t), "their work is untouched")
	assert.Empty(t, p.gh.opened)
}

func TestAPullRequestThatCannotBeOpenedIsSaidSoAndTheTokenStaysOut(t *testing.T) {
	p := newPublishing(t, writes("Did it.", 0))
	p.gh.url, p.gh.err = "", errors.New("open a pull request: status 500 "+pushToken)

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Empty(t, got.prURL)
	assert.Contains(t, got.outcome, "the branch was pushed, but its pull request")
	assert.NotContains(t, got.outcome, pushToken)
	assert.NotContains(t, p.logs.String(), pushToken, "scrubbed from the log too")
	assert.NotEmpty(t, p.remoteBranch(t))
}

func TestPushOutputThatEchoesTheTokenIsScrubbed(t *testing.T) {
	p := newPublishing(t, writes("Did it.", 0))
	p.runner.Git.RunWithEnv = func(
		context.Context, string, []string, ...string,
	) ([]byte, error) {
		return []byte("fatal: could not read from " + pushToken), errors.New("exit 128")
	}
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, p.logs.String(), pushToken)
	assert.NotContains(t, p.api.done()[0].outcome, pushToken)
}

func TestScrubRemovesTheTokenAndItsBasicAuthForm(t *testing.T) {
	assert.Equal(
		t,
		"a [redacted] b",
		runner.ScrubForTest("a tok-abcdef-123 b", "tok-abcdef-123"),
	)
	header := strings.Join(runner.PushEnvForTest("tok-abcdef-123"), " ")
	basic := header[strings.Index(header, "basic ")+len("basic "):]
	assert.Equal(t, "x [redacted]", runner.ScrubForTest("x "+basic, "tok-abcdef-123"))
	assert.Equal(t, "plain", runner.ScrubForTest("plain", ""))
}

func TestPullRequestTextTakesTheTitleFromTheFirstLine(t *testing.T) {
	job := &runner.ClaimedJob{
		ID: "job-1", TaskID: "task-1", Branch: "agent/x",
		RepoOwner: "", RepoName: "", DefaultBranch: "", Harness: "", Prompt: "",
		SetupCommands: "", TimeLimitMinutes: 0, MCPClientID: "", MCPClientSecret: "",
	}
	for name, tc := range map[string]struct{ outcome, title string }{
		"plain":            {"Fix it\n\nbody", "Fix it"},
		"labelled":         {"PR title: Fix it\n\nbody", "Fix it"},
		"labelled lower":   {"pr title : Fix it", "Fix it"},
		"heading":          {"## Fix it\nbody", "Fix it"},
		"labelled heading": {"# Title: Fix it\nbody", "Fix it"},
		"quoted":           {"\"Fix it\"", "Fix it"},
		"blank first":      {"\n\n  Fix it  \nbody", "Fix it"},
		"spaces":           {"Fix    the   it", "Fix the it"},
		"empty":            {"", "Factory job on agent/x"},
		"only a label":     {"PR title:", "Factory job on agent/x"},
	} {
		title, body := runner.PullRequestTextForTest(tc.outcome, job, "")
		assert.Equal(t, tc.title, title, name)
		assert.Contains(t, body, "task `task-1` (job `job-1`)", name)
		assert.NotContains(t, body, "Board:", name+": no server address, no link")
	}

	long := strings.Repeat("é", 300)
	title, _ := runner.PullRequestTextForTest(long, job, "")
	assert.Len(t, []rune(title), 200)

	_, body := runner.PullRequestTextForTest(strings.Repeat("x", 70000), job, "https://s/")
	assert.Less(t, len([]rune(body)), 60500)
	assert.Contains(t, body, "…")
	assert.Contains(t, body, "Board: https://s/factory")
	assert.Contains(t, body, "never merges")
}

func TestTheGitHubClientOpensAPullRequest(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/repos/xdoubleu/tools.xdoubleu.com/pulls", r.URL.Path)
			assert.Equal(t, "Bearer "+pushToken, r.Header.Get("Authorization"))
			assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(body, &got))
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"html_url":"https://github.com/o/r/pull/9"}`)
		}),
	)
	t.Cleanup(srv.Close)
	c := &runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}

	url, err := c.OpenPullRequest(t.Context(), pushToken, runner.PullRequest{
		Owner: "xdoubleu", Repo: "tools.xdoubleu.com", Head: "agent/x", Base: "main",
		Title: "T", Body: "B",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/o/r/pull/9", url)
	assert.Equal(t, map[string]any{
		"title": "T", "head": "agent/x", "base": "main", "body": "B",
		"maintainer_can_modify": false,
	}, got)
}

func TestTheGitHubClientReusesTheOpenPullRequestOfABranch(t *testing.T) {
	var listed string
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(
					w,
					`{"message":"Validation Failed","errors":`+
						`[{"message":"A pull request already exists for o:agent/x."}]}`,
				)
				return
			}
			listed = r.URL.RawQuery
			_, _ = io.WriteString(w, `[{"html_url":"https://github.com/o/r/pull/3"}]`)
		}),
	)
	t.Cleanup(srv.Close)
	c := &runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}

	url, err := c.OpenPullRequest(t.Context(), "t", runner.PullRequest{
		Owner: "o", Repo: "r", Head: "agent/x", Base: "main", Title: "", Body: "",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/o/r/pull/3", url)
	assert.Equal(t, "head=o%3Aagent%2Fx&per_page=1&state=open", listed)
}

func TestTheGitHubClientFailsWithoutLeakingGitHubsText(t *testing.T) {
	answer := func(status int, body string) *runner.GitHubClient {
		srv := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}),
		)
		t.Cleanup(srv.Close)
		return &runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}
	}
	pr := runner.PullRequest{
		Owner: "o", Repo: "r", Head: "h", Base: "b", Title: "", Body: "",
	}

	_, err := answer(http.StatusForbidden, `{"message":"secret detail"}`).
		OpenPullRequest(t.Context(), "t", pr)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret detail")
	assert.Contains(t, err.Error(), "status 403")

	// 422 for another reason is just a failure.
	_, err = answer(http.StatusUnprocessableEntity, `{"message":"No commits between"}`).
		OpenPullRequest(t.Context(), "t", pr)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "No commits")

	for name, c := range map[string]*runner.GitHubClient{
		"not json":     answer(http.StatusCreated, `nope`),
		"no address":   answer(http.StatusCreated, `{}`),
		"unreachable":  {BaseURL: "http://127.0.0.1:1", HTTP: http.DefaultClient},
		"bad base URL": {BaseURL: "://", HTTP: http.DefaultClient},
	} {
		_, err = c.OpenPullRequest(t.Context(), "t", pr)
		assert.Error(t, err, name)
	}

	// The open pull request can't be found after "already exists".
	exists := `{"errors":[{"message":"A pull request already exists"}]}`
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(w, exists)
				return
			}
			_, _ = io.WriteString(w, `[]`)
		}),
	)
	t.Cleanup(srv.Close)
	_, err = (&runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}).
		OpenPullRequest(t.Context(), "t", pr)
	require.ErrorContains(t, err, "none found")

	broken := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(w, exists)
				return
			}
			w.WriteHeader(http.StatusBadGateway)
		}),
	)
	t.Cleanup(broken.Close)
	_, err = (&runner.GitHubClient{BaseURL: broken.URL, HTTP: broken.Client()}).
		OpenPullRequest(t.Context(), "t", pr)
	require.ErrorContains(t, err, "find the open pull request")
}

func TestNewGitHubClientTalksToGitHub(t *testing.T) {
	c := runner.NewGitHubClient()
	assert.Equal(t, "https://api.github.com", c.BaseURL)
	assert.NotNil(t, c.HTTP)
}

func TestAJobIsPushedByTheRealGitToo(t *testing.T) {
	// No test hook: the push runs git itself, with the token in its environment.
	env := newEnv(t, writes("Real push", 0), newJob())
	gh := &fakeGitHub{
		mu: sync.Mutex{}, opened: nil, tokens: nil, err: nil,
		url: "https://github.com/xdoubleu/tools.xdoubleu.com/pull/8",
	}
	env.api.pushToken = pushToken
	env.runner.GitHub = gh

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.Equal(t, "https://github.com/xdoubleu/tools.xdoubleu.com/pull/8", got.prURL)
	out, err := exec.Command("git", "-C", env.remote, "rev-parse", "--verify", "-q",
		"refs/heads/"+branch).CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestGitWithEnvironmentUsesTheHooksInOrderOfPreference(t *testing.T) {
	ctx := context.Background()
	var sawEnv []string
	withEnv := &runner.Git{
		Dir: "", RemoteURL: nil, Run: nil,
		RunWithEnv: func(
			_ context.Context, _ string, env []string, _ ...string,
		) ([]byte, error) {
			sawEnv = env
			return []byte("env"), nil
		},
	}
	out, err := withEnv.GitEnvForTest(ctx, "", []string{"A=1"}, "x")
	require.NoError(t, err)
	assert.Equal(t, "env", string(out))
	assert.Equal(t, []string{"A=1"}, sawEnv)

	plain := &runner.Git{
		Dir: "", RemoteURL: nil, RunWithEnv: nil,
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			return []byte(strings.Join(args, " ")), nil
		},
	}
	out, err = plain.GitEnvForTest(ctx, "", []string{"A=1"}, "a", "b")
	require.NoError(t, err)
	assert.Equal(t, "a b", string(out))

	actual := &runner.Git{Dir: "", RemoteURL: nil, Run: nil, RunWithEnv: nil}
	out, err = actual.GitEnvForTest(ctx, t.TempDir(), []string{"GIT_AUTHOR_NAME=zed"},
		"var", "GIT_AUTHOR_IDENT")
	require.NoError(t, err)
	assert.Contains(t, string(out), "zed", "the environment reaches git")
}
