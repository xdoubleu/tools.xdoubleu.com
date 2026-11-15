package obs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/reposignals"
	"tools.xdoubleu.com/internal/platform/sentryapi"
)

type fakeRepos struct {
	repos []reposignals.Repo
	err   error
}

func (f fakeRepos) MonitoredRepos(context.Context) ([]reposignals.Repo, error) {
	return f.repos, f.err
}

// fakeGithub embeds the interface so only the calls under test are defined.
type fakeGithub struct {
	github.Client

	byRepo map[string][]github.PullRequest
	picked []github.PullRequest
	scopes map[string]github.PRScope
}

func (f fakeGithub) ListFailingPullRequests(
	context.Context,
) ([]github.PullRequest, error) {
	return f.picked, nil
}

func (f fakeGithub) ListFailingPullRequestsIn(
	_ context.Context, repo string, scope github.PRScope,
) ([]github.PullRequest, error) {
	f.scopes[repo] = scope
	return f.byRepo[repo], nil
}

type fakeSentry struct {
	sentryapi.Client

	issues []sentryapi.Issue
}

func (f fakeSentry) ListUnresolvedIssues(
	context.Context,
) ([]sentryapi.Issue, error) {
	return f.issues, nil
}

func signalRepos() fakeRepos {
	return fakeRepos{
		repos: []reposignals.Repo{
			{
				Slug: "o/tools", SentryProject: "tools",
				RedPRLabel: "dependencies", RedPRBranchPrefix: "claude/",
			},
			{
				Slug: "o/check-in", SentryProject: "check-in",
				RedPRLabel: "", RedPRBranchPrefix: "agent/",
			},
		},
		err: nil,
	}
}

func signalHandler(repos reposignals.Source, gh github.Client, s sentryapi.Client,
) *obsConnectHandler {
	//nolint:exhaustruct //only these dependencies are read
	return &obsConnectHandler{deps: &Deps{
		Logger:       logging.NewNopLogger(),
		Repos:        repos,
		GithubClient: gh,
		SentryClient: s,
	}}
}

func newFakeGithub() fakeGithub {
	return fakeGithub{
		Client: nil,
		byRepo: map[string][]github.PullRequest{
			"o/tools": {
				//nolint:exhaustruct //only the number identifies the PR
				{Number: 1},
			},
			"o/check-in": {
				//nolint:exhaustruct //only the number identifies the PR
				{Number: 2},
			},
		},
		picked: nil,
		scopes: map[string]github.PRScope{},
	}
}

func TestFailingPullRequestsCoverEveryRepoWithItsOwnScope(t *testing.T) {
	gh := newFakeGithub()
	h := signalHandler(signalRepos(), gh, nil)

	resp, err := h.failingPullRequests(t.Context(), "")

	require.NoError(t, err)
	require.Len(t, resp.GetPullRequests(), 2)
	assert.EqualValues(t, 2, resp.GetFailingCount())
	assert.Equal(t, "o/tools", resp.GetPullRequests()[0].GetRepo())
	assert.Equal(t, "o/check-in", resp.GetPullRequests()[1].GetRepo())
	assert.Equal(t,
		github.PRScope{Label: "", BranchPrefix: "agent/"}, gh.scopes["o/check-in"])
}

func TestFailingPullRequestsRepoFilter(t *testing.T) {
	h := signalHandler(signalRepos(), newFakeGithub(), nil)

	resp, err := h.failingPullRequests(t.Context(), "O/Check-In")
	require.NoError(t, err)
	require.Len(t, resp.GetPullRequests(), 1)
	assert.EqualValues(t, 2, resp.GetPullRequests()[0].GetNumber())

	_, err = h.failingPullRequests(t.Context(), "o/unknown")
	require.Error(t, err)
}

func TestFailingPullRequestsFallBackToThePickedRepo(t *testing.T) {
	gh := newFakeGithub()
	//nolint:exhaustruct //only the number identifies the PR
	gh.picked = []github.PullRequest{{Number: 7}}
	h := signalHandler(fakeRepos{repos: nil, err: errors.New("db down")}, gh, nil)

	resp, err := h.failingPullRequests(t.Context(), "")

	require.NoError(t, err)
	require.Len(t, resp.GetPullRequests(), 1)
	assert.Empty(t, resp.GetPullRequests()[0].GetRepo())

	_, err = h.failingPullRequests(t.Context(), "o/tools")
	require.Error(t, err, "a repo filter needs registered repos")
}

func TestSentryIssuesAreTaggedAndFilteredByRepo(t *testing.T) {
	s := fakeSentry{
		Client: nil,
		issues: []sentryapi.Issue{
			sentryIssue("1", "tools"), sentryIssue("2", "check-in"),
			sentryIssue("3", "other"),
		},
	}
	h := signalHandler(signalRepos(), nil, s)

	all := h.sentryIssues(t.Context(), "")
	require.Len(t, all.GetIssues(), 3)
	assert.Equal(t, "o/tools", all.GetIssues()[0].GetRepo())
	assert.Equal(t, "o/check-in", all.GetIssues()[1].GetRepo())
	assert.Empty(t, all.GetIssues()[2].GetRepo())

	one := h.sentryIssues(t.Context(), "o/check-in")
	require.Len(t, one.GetIssues(), 1)
	assert.Equal(t, "2", one.GetIssues()[0].GetId())
	assert.EqualValues(t, 1, one.GetUnresolvedCount())
}

func sentryIssue(id, project string) sentryapi.Issue {
	//nolint:exhaustruct //only the id and project matter here
	return sentryapi.Issue{ID: id, Project: project}
}
