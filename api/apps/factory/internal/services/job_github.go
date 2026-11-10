package services

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/githubapp"
)

// ErrPushNotConfigured is returned when no GitHub App is set up, so a job's work
// stays on the runner's Mac and its task goes to Needs you as before.
var ErrPushNotConfigured = errors.New("pushing is not configured")

// JobGitHub mints the token a runner pushes a job's branch with (adr-0045). It is
// optional: without it no job can push.
type JobGitHub interface {
	InstallationToken(ctx context.Context, owner, repo string) (githubapp.Token, error)
}

// PullRequestSource reads where a pull request stands (adr-0045).
type PullRequestSource interface {
	PullRequest(
		ctx context.Context,
		owner, repo string,
		number int,
	) (githubapp.PullRequestStatus, error)
	// MergedPullRequests lists the recently merged pull requests of a repo.
	MergedPullRequests(
		ctx context.Context,
		owner, repo string,
	) ([]githubapp.MergedPullRequest, error)
}

// GitHub is everything the factory asks of the GitHub App.
type GitHub interface {
	JobGitHub
	PullRequestSource
}

// ErrGitHubFailed is any failure to get a token from GitHub other than the App
// not being installed: worth retrying, and with none of GitHub's text.
var ErrGitHubFailed = errors.New("could not get a token from GitHub")

// PushToken is a short-lived token for the repo of a running job: contents and
// pull requests write on that repo alone. Only the runner that claimed the job
// may ask for it.
func (s *JobService) PushToken(
	ctx context.Context,
	runnerID, jobID uuid.UUID,
) (githubapp.Token, error) {
	if s.github == nil {
		return githubapp.Token{}, ErrPushNotConfigured
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return githubapp.Token{}, err
	}
	if job.Status != models.JobRunning {
		return githubapp.Token{}, conflict("the job is no longer running")
	}
	if job.RunnerID == nil || *job.RunnerID != runnerID {
		return githubapp.Token{}, forbidden("the job was claimed by another runner")
	}
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return githubapp.Token{}, err
	}
	token, err := s.github.InstallationToken(ctx, repo.Owner, repo.Name)
	if errors.Is(err, githubapp.ErrNotInstalled) {
		return githubapp.Token{}, err
	}
	if err != nil {
		return githubapp.Token{}, ErrGitHubFailed
	}
	return token, nil
}

var pullPath = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull/([1-9][0-9]*)$`)

// pullNumber is the number of raw, if it is the address of a pull request of the
// repo, as GitHub writes it. The runner reports it, so it is checked: a link on a
// task must lead to GitHub and to this repo's pulls, nowhere else.
func pullNumber(raw string, repo *models.Repo) (int, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return 0, false
	}
	m := pullPath.FindStringSubmatch(u.Path)
	if m == nil || !strings.EqualFold(m[1], repo.Owner) ||
		!strings.EqualFold(m[2], repo.Name) {
		return 0, false
	}
	n, err := strconv.Atoi(m[3])
	return n, err == nil
}

func ownPullRequest(raw string, repo *models.Repo) bool {
	_, ok := pullNumber(raw, repo)
	return ok
}
