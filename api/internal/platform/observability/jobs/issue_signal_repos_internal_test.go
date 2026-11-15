package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/reposignals"
	"tools.xdoubleu.com/internal/platform/sentryapi"
)

type stubRepoSource struct {
	repos []reposignals.Repo
	err   error
}

func (s stubRepoSource) MonitoredRepos(
	_ context.Context,
) ([]reposignals.Repo, error) {
	return s.repos, s.err
}

// scopedGithubClient answers the per-repo listing from byRepo, recording the
// scope each repo was asked with.
type scopedGithubClient struct {
	stubGithubClient

	byRepo map[string][]github.PullRequest
	failOn string
	scopes map[string]github.PRScope
}

func (g scopedGithubClient) ListFailingPullRequestsIn(
	_ context.Context, repo string, scope github.PRScope,
) ([]github.PullRequest, error) {
	g.scopes[repo] = scope
	if repo == g.failOn {
		return nil, errors.New("boom")
	}
	return g.byRepo[repo], nil
}

func twoRepos() stubRepoSource {
	return stubRepoSource{
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

func TestCollectFailingPullRequestsPerRepo(t *testing.T) {
	resetGauges()
	gh := scopedGithubClient{
		//nolint:exhaustruct //only the per-repo listing is read
		stubGithubClient: stubGithubClient{},
		byRepo: map[string][]github.PullRequest{
			"o/tools":    failingPRs(1),
			"o/check-in": failingPRs(2),
		},
		failOn: "",
		scopes: map[string]github.PRScope{},
	}
	//nolint:exhaustruct //only the github client and repos are read
	job := (&IssueSignalCollectorJob{gh: gh}).WithRepos(twoRepos())
	logger, _ := loggerWithBuf()

	job.collectFailingPullRequests(t.Context(), logger)

	assert.InDelta(t, 3.0, testutil.ToFloat64(githubFailingPullRequests), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		githubFailingPullRequestsByRepo.WithLabelValues("o/tools")), 0)
	assert.InDelta(t, 2.0, testutil.ToFloat64(
		githubFailingPullRequestsByRepo.WithLabelValues("o/check-in")), 0)
	assert.Equal(t,
		github.PRScope{Label: "", BranchPrefix: "agent/"}, gh.scopes["o/check-in"])
}

func TestCollectFailingPullRequestsOneRepoFailingKeepsGauges(t *testing.T) {
	resetGauges()
	githubFailingPullRequests.Set(9)
	gh := scopedGithubClient{
		//nolint:exhaustruct //only the per-repo listing is read
		stubGithubClient: stubGithubClient{},
		byRepo:           map[string][]github.PullRequest{"o/tools": failingPRs(1)},
		failOn:           "o/check-in",
		scopes:           map[string]github.PRScope{},
	}
	//nolint:exhaustruct //only the github client and repos are read
	job := (&IssueSignalCollectorJob{gh: gh}).WithRepos(twoRepos())
	logger, _ := loggerWithBuf()

	job.collectFailingPullRequests(t.Context(), logger)

	assert.InDelta(t, 9.0, testutil.ToFloat64(githubFailingPullRequests), 0)
}

func TestCollectFailingPullRequestsFallsBackWithoutRepos(t *testing.T) {
	resetGauges()
	//nolint:exhaustruct //only prs drives the fallback listing
	gh := stubGithubClient{prs: failingPRs(4)}
	//nolint:exhaustruct //only the github client and repos are read
	job := (&IssueSignalCollectorJob{gh: gh}).
		WithRepos(stubRepoSource{repos: nil, err: errors.New("db down")})
	logger, _ := loggerWithBuf()

	job.collectFailingPullRequests(t.Context(), logger)

	assert.InDelta(t, 4.0, testutil.ToFloat64(githubFailingPullRequests), 0)
}

func TestCollectSentryIssuesByRepo(t *testing.T) {
	resetGauges()
	sentry := stubSentryClient{
		issues: []sentryapi.Issue{
			issueIn("tools"), issueIn("tools"), issueIn("check-in"), issueIn("other"),
		},
		err: nil,
	}
	//nolint:exhaustruct //only the sentry client and repos are read
	job := (&IssueSignalCollectorJob{sentry: sentry}).WithRepos(twoRepos())
	logger, _ := loggerWithBuf()

	job.collectSentryIssues(t.Context(), logger)

	assert.InDelta(t, 4.0, testutil.ToFloat64(sentryUnresolvedIssues), 0)
	assert.InDelta(t, 2.0, testutil.ToFloat64(
		sentryUnresolvedIssuesByRepo.WithLabelValues("o/tools")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		sentryUnresolvedIssuesByRepo.WithLabelValues("o/check-in")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		sentryUnresolvedIssuesByRepo.WithLabelValues(unassignedRepo)), 0)
}

func issueIn(project string) sentryapi.Issue {
	//nolint:exhaustruct //only the project attributes an issue to a repo
	return sentryapi.Issue{Project: project}
}
