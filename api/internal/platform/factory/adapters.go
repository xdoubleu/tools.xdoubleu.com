package factory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/githubapp"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
)

// GitHub is everything the factory asks of the GitHub App.
type GitHub interface {
	InstallationToken(ctx context.Context, owner, repo string) (githubapp.Token, error)
	PullRequest(
		ctx context.Context,
		owner, repo string,
		number int,
	) (githubapp.PullRequestStatus, error)
	MergedPullRequests(
		ctx context.Context,
		owner, repo string,
	) ([]githubapp.MergedPullRequest, error)
	OpenPullRequests(
		ctx context.Context,
		owner, repo string,
	) ([]githubapp.OpenPullRequest, error)
	PullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error)
	Deployed(ctx context.Context, owner, repo, sha, release string) (bool, error)
}

// JobAccounts makes and ends the agent account a job runs as.
type JobAccounts interface {
	CreateForJob(
		ctx context.Context,
		createdBy, name string,
		scope agentaccounts.Scope,
	) (*agentaccounts.Account, string, error)
	RevokeForJob(ctx context.Context, jobID uuid.UUID) error
	ActiveJobs(ctx context.Context) ([]uuid.UUID, error)
	ScopeOf(ctx context.Context, userID uuid.UUID) (*agentaccounts.Scope, error)
	OwnerNames(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error)
}

func newStores(r *repositories.Repositories) services.Stores {
	return services.Stores{
		Tasks: r.Tasks, Repos: r.Repos, Runners: r.Runners, Jobs: r.Jobs,
		Events: r.Events, Settings: r.Settings, Linked: r.Linked,
		Routines: r.Routines,
	}
}

// githubPort adapts the GitHub App client to the services' port.
type githubPort struct{ gh GitHub }

var _ services.GitHub = githubPort{nil}

// adaptGitHub is nil for no client, which turns pushing off.
func adaptGitHub(gh GitHub) services.GitHub {
	if gh == nil {
		return nil
	}
	return githubPort{gh: gh}
}

func mapGitHubErr(err error) error {
	switch {
	case errors.Is(err, githubapp.ErrNotInstalled):
		return services.ErrNotInstalled
	case errors.Is(err, githubapp.ErrPullRequestGone):
		return services.ErrPullRequestGone
	}
	return err
}

func (p githubPort) InstallationToken(
	ctx context.Context,
	owner, repo string,
) (services.Token, error) {
	t, err := p.gh.InstallationToken(ctx, owner, repo)
	return services.Token{Value: t.Value, ExpiresAt: t.ExpiresAt}, mapGitHubErr(err)
}

func (p githubPort) PullRequest(
	ctx context.Context,
	owner, repo string,
	number int,
) (services.PullRequestStatus, error) {
	s, err := p.gh.PullRequest(ctx, owner, repo, number)
	return services.PullRequestStatus{
		State: s.State, CI: s.CI, Review: s.Review,
		Mergeable: s.Mergeable, MergeSHA: s.MergeSHA,
	}, mapGitHubErr(err)
}

func (p githubPort) MergedPullRequests(
	ctx context.Context,
	owner, repo string,
) ([]services.MergedPullRequest, error) {
	pulls, err := p.gh.MergedPullRequests(ctx, owner, repo)
	out := make([]services.MergedPullRequest, len(pulls))
	for i, pr := range pulls {
		out[i] = services.MergedPullRequest(pr)
	}
	return out, mapGitHubErr(err)
}

func (p githubPort) OpenPullRequests(
	ctx context.Context,
	owner, repo string,
) ([]services.OpenPullRequest, error) {
	pulls, err := p.gh.OpenPullRequests(ctx, owner, repo)
	out := make([]services.OpenPullRequest, len(pulls))
	for i, pr := range pulls {
		out[i] = services.OpenPullRequest(pr)
	}
	return out, mapGitHubErr(err)
}

func (p githubPort) PullRequestFiles(
	ctx context.Context,
	owner, repo string,
	number int,
) ([]string, error) {
	files, err := p.gh.PullRequestFiles(ctx, owner, repo, number)
	return files, mapGitHubErr(err)
}

func (p githubPort) Deployed(
	ctx context.Context,
	owner, repo, sha, release string,
) (bool, error) {
	ok, err := p.gh.Deployed(ctx, owner, repo, sha, release)
	return ok, mapGitHubErr(err)
}

// accountsPort adapts the agent accounts store to the services' port.
type accountsPort struct{ accounts JobAccounts }

var _ services.JobAccounts = accountsPort{nil}

// adaptAccounts is nil for no store, which gives jobs no MCP identity.
func adaptAccounts(accounts JobAccounts) services.JobAccounts {
	if accounts == nil {
		return nil
	}
	return accountsPort{accounts: accounts}
}

func (p accountsPort) CreateForJob(
	ctx context.Context,
	createdBy, name string,
	scope services.JobScope,
) (string, string, error) {
	account, secret, err := p.accounts.CreateForJob(ctx, createdBy, name,
		agentaccounts.Scope(scope))
	if err != nil {
		return "", "", err
	}
	return account.ClientID, secret, nil
}

func (p accountsPort) RevokeForJob(ctx context.Context, jobID uuid.UUID) error {
	return p.accounts.RevokeForJob(ctx, jobID)
}

func (p accountsPort) ActiveJobs(ctx context.Context) ([]uuid.UUID, error) {
	return p.accounts.ActiveJobs(ctx)
}

func (p accountsPort) ScopeOf(
	ctx context.Context,
	userID uuid.UUID,
) (*services.JobScope, error) {
	scope, err := p.accounts.ScopeOf(ctx, userID)
	if err != nil || scope == nil {
		return nil, err
	}
	return &services.JobScope{JobID: scope.JobID, TaskID: scope.TaskID}, nil
}

func (p accountsPort) OwnerNames(
	ctx context.Context,
	userIDs []uuid.UUID,
) (map[uuid.UUID]string, error) {
	return p.accounts.OwnerNames(ctx, userIDs)
}

const (
	// jobImageManifest is the manifest build-api stages under /downloads/.
	jobImageManifest = "factory-job-image.txt"
	maxManifestBytes = 1 << 10
	// manifestTimeout bounds one manifest download.
	manifestTimeout = 30 * time.Second
)

// manifestDigest reads the digest from the manifest served under webURL: a
// first-party URL from config, not user input.
func manifestDigest(webURL string, client *http.Client) services.DigestSource {
	return func(ctx context.Context) (string, error) {
		url := strings.TrimRight(webURL, "/") + "/downloads/" + jobImageManifest
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("job image manifest: %s", resp.Status)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes))
		if err != nil {
			return "", err
		}
		release, digest, ok := strings.Cut(strings.TrimSpace(string(raw)), " ")
		if !ok || release == "" || strings.ContainsAny(digest, " \n") || digest == "" {
			return "", errors.New("job image manifest is malformed")
		}
		return digest, nil
	}
}
