package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// Pull request, CI and review states, as the factory shows them.
const (
	StateOpen   = "open"
	StateMerged = "merged"
	StateClosed = "closed"
)

// ErrNotInstalled is returned when the GitHub App isn't installed on the repository.
var ErrNotInstalled = errors.New("github app is not installed on the repository")

// ErrPullRequestGone is returned when GitHub has no such pull request.
var ErrPullRequestGone = errors.New("github: pull request not found")

// Token is an installation access token and when GitHub stops honouring it.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// PullRequestStatus is where a pull request stands. CI and Review are empty when
// there is nothing to report (no checks, no reviews) or once it is no longer open.
type PullRequestStatus struct {
	State  string
	CI     string
	Review string
	// Mergeable is "conflicting" or "behind" when auto-merge is armed and the
	// pull request still can't merge for that reason; empty otherwise.
	Mergeable string
	// MergeSHA is the commit a merged pull request produced; empty otherwise.
	MergeSHA string
}

// MergedPullRequest is a merged pull request, with the title, branch and body its
// tasks are linked from.
type MergedPullRequest struct {
	URL    string
	Title  string
	Branch string
	Body   string
	// MergeSHA is the commit the merge produced on the base branch.
	MergeSHA string
}

// OpenPullRequest is an open pull request, with the title, branch and body its
// tasks are linked from.
type OpenPullRequest struct {
	URL    string
	Number int
	Title  string
	Branch string
	Body   string
}

// ErrPushNotConfigured is returned when no GitHub App is set up, so a job's work
// stays on the runner's Mac and its task goes to Needs you as before.
var ErrPushNotConfigured = errors.New("pushing is not configured")

// JobGitHub mints the token a runner pushes a job's branch with (adr-0045). It is
// optional: without it no job can push.
type JobGitHub interface {
	InstallationToken(ctx context.Context, owner, repo string) (Token, error)
}

// PullRequestSource reads where a pull request stands (adr-0045).
type PullRequestSource interface {
	PullRequest(
		ctx context.Context,
		owner, repo string,
		number int,
	) (PullRequestStatus, error)
	// MergedPullRequests lists the recently merged pull requests of a repo.
	MergedPullRequests(
		ctx context.Context,
		owner, repo string,
	) ([]MergedPullRequest, error)
	// OpenPullRequests lists the open pull requests of a repo.
	OpenPullRequests(
		ctx context.Context,
		owner, repo string,
	) ([]OpenPullRequest, error)
	// PullRequestFiles lists the paths a pull request changes.
	PullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error)
	// Deployed reports whether release contains the commit sha.
	Deployed(ctx context.Context, owner, repo, sha, release string) (bool, error)
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
) (Token, error) {
	if s.github == nil {
		return Token{}, ErrPushNotConfigured
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return Token{}, err
	}
	if job.Status != models.JobRunning {
		return Token{}, conflict("the job is no longer running")
	}
	if job.RunnerID == nil || *job.RunnerID != runnerID {
		return Token{}, forbidden("the job was claimed by another runner")
	}
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return Token{}, err
	}
	token, err := s.github.InstallationToken(ctx, repo.Owner, repo.Name)
	if errors.Is(err, ErrNotInstalled) {
		return Token{}, err
	}
	if err != nil {
		return Token{}, fmt.Errorf("%w: %w", ErrGitHubFailed, err)
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
