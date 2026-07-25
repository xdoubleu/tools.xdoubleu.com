package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.github.com"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 15 * time.Second

const (
	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
	// cacheTTL is how long a fetched issue list is served from memory before
	// the next call re-fetches. Keeps the admin dashboard off the API rate
	// limit while staying fresh enough for observability.
	cacheTTL = 45 * time.Second
)

// configStore is the subset of *repositories.OAuthConnectionsRepository used
// to resolve the admin-picked repo fresh on every call, instead of a static
// value baked in at boot (mirrors oauthconn's own narrow connectionStore).
type configStore interface {
	Get(
		ctx context.Context, provider models.OAuthProvider,
	) (*oauth2.Token, *models.OAuthConnection, error)
}

type repoConfig struct {
	Repo string `json:"repo"`
}

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	tokenFn    oauthconn.TokenFunc
	configRepo configStore

	mu         sync.Mutex
	cached     []Issue
	cachedAt   time.Time
	cachedPRs  []PullRequest
	cachedPRAt time.Time
}

// New creates a GitHub client. tokenFn resolves a live OAuth bearer token
// (see internal/oauthconn) and configRepo resolves the admin-picked
// "owner/name" repo on every call. When no repo is picked, or tokenFn
// reports the provider isn't connected, every call returns ErrNotConfigured.
func New(
	logger *slog.Logger, tokenFn oauthconn.TokenFunc, configRepo configStore,
) Client {
	return &client{ //nolint:exhaustruct // cache fields start zero-valued
		logger:     logger,
		httpClient: &http.Client{Timeout: apiTimeout},
		tokenFn:    tokenFn,
		configRepo: configRepo,
	}
}

func (c *client) ListOpenIssues(ctx context.Context) ([]Issue, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedIssues(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	issues, err := c.fetch(ctx, token, repo)
	if err != nil {
		return nil, err
	}

	c.store(issues)
	return issues, nil
}

func (c *client) ListFailingPullRequests(ctx context.Context) ([]PullRequest, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedPullRequests(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	prs, err := c.fetchFailingPullRequests(ctx, token, repo)
	if err != nil {
		return nil, err
	}

	c.storePullRequests(prs)
	return prs, nil
}

// resolveRepo reads the admin-picked repo from the stored connection config.
// Returns ErrNotConfigured when the provider isn't connected or no repo has
// been picked yet.
func (c *client) resolveRepo(ctx context.Context) (string, error) {
	_, conn, err := c.configRepo.Get(ctx, models.OAuthProviderGithub)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", ErrNotConfigured
	}
	if err != nil {
		return "", err
	}
	if len(conn.Config) == 0 {
		return "", ErrNotConfigured
	}

	var cfg repoConfig
	if unmarshalErr := json.Unmarshal(conn.Config, &cfg); unmarshalErr != nil {
		return "", unmarshalErr
	}
	if cfg.Repo == "" {
		return "", ErrNotConfigured
	}
	return cfg.Repo, nil
}

func (c *client) cachedIssues() ([]Issue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil && time.Since(c.cachedAt) < cacheTTL {
		return c.cached, true
	}
	return nil, false
}

func (c *client) store(issues []Issue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = issues
	c.cachedAt = time.Now()
}

func (c *client) cachedPullRequests() ([]PullRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedPRs != nil && time.Since(c.cachedPRAt) < cacheTTL {
		return c.cachedPRs, true
	}
	return nil, false
}

func (c *client) storePullRequests(prs []PullRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedPRs = prs
	c.cachedPRAt = time.Now()
}

func (c *client) fetch(ctx context.Context, token, repo string) ([]Issue, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/issues?state=open", baseURL, repo)

	var wires []issueWire
	if err := c.get(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(wires))
	for _, w := range wires {
		if w.PullRequest != nil {
			continue // the /issues endpoint returns PRs too — skip them
		}
		issues = append(issues, w.toIssue())
	}
	return issues, nil
}

// fetchFailingPullRequests lists the repo's open pull requests and, for each,
// fetches the check runs on its head commit. Only pull requests with at least
// one failing check run are returned.
func (c *client) fetchFailingPullRequests(
	ctx context.Context, token, repo string,
) ([]PullRequest, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/pulls?state=open", baseURL, repo)

	var wires []prWire
	if err := c.get(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	prs := make([]PullRequest, 0, len(wires))
	for _, w := range wires {
		checks, err := c.fetchFailingChecks(ctx, token, repo, w.Head.SHA)
		if err != nil {
			return nil, err
		}
		if len(checks) == 0 {
			continue
		}
		prs = append(prs, PullRequest{
			Number:        w.Number,
			Title:         w.Title,
			URL:           w.HTMLURL,
			Author:        w.User.Login,
			UpdatedAt:     w.UpdatedAt,
			FailingChecks: checks,
		})
	}
	return prs, nil
}

// fetchFailingChecks returns the non-passing, completed check runs on sha.
func (c *client) fetchFailingChecks(
	ctx context.Context, token, repo, sha string,
) ([]FailingCheck, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/commits/%s/check-runs", baseURL, repo, sha)

	var wire checkRunsWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, err
	}

	checks := make([]FailingCheck, 0, len(wire.CheckRuns))
	for _, run := range wire.CheckRuns {
		if run.Status != "completed" || !failingConclusions[run.Conclusion] {
			continue
		}
		checks = append(checks, FailingCheck{
			Name:       run.Name,
			Conclusion: run.Conclusion,
			URL:        run.HTMLURL,
		})
	}
	return checks, nil
}

func (c *client) get(ctx context.Context, endpoint, token string, dst any) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodGet, endpoint, nil,
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, fmt.Errorf(
				"github API returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"github API returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		return false, json.NewDecoder(resp.Body).Decode(dst)
	})
}

// doWithRetry calls attempt up to maxAttempts times with exponential backoff.
func (c *client) doWithRetry(
	ctx context.Context,
	attempt func() (retryable bool, err error),
) error {
	var lastErr error
	for i := range maxAttempts {
		retryable, err := attempt()
		if err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) {
			return err
		}

		lastErr = err

		if !retryable || i == maxAttempts-1 {
			break
		}

		delay := backoffDelay(i)
		c.logger.DebugContext(ctx, "retrying github request",
			slog.Int("attempt", i+1),
			slog.Duration("backoff", delay),
			slog.Any("error", err),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// SetBaseURL overrides the GitHub API base URL. Intended for tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the exponential-backoff base delay. Intended for
// tests only so retry tests run without real wall-clock sleeps.
func SetBackoffBase(d time.Duration) { backoffBase = d }

func backoffDelay(attempt int) time.Duration {
	d := backoffBase * (1 << attempt)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		(status >= http.StatusInternalServerError && status < 600)
}

func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Timeout()
	}
	return false
}
