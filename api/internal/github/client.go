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

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/database"
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

// statusCompleted is the GitHub Actions "completed" run/job status.
const statusCompleted = "completed"

// errNotFound wraps a 404 response from get, so getAllowingNotFound can
// distinguish "endpoint not enabled for this repo" from a real failure.
var errNotFound = errors.New("github: not found")

const (
	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
	// cacheTTL is how long a fetched pull-request list is served from memory
	// before the next call re-fetches. Keeps the admin dashboard off the API
	// rate limit while staying fresh enough for observability.
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

	mu            sync.Mutex
	cachedPRs     []PullRequest
	cachedPRAt    time.Time
	cachedAlerts  []SecurityAlert
	cachedAlertAt time.Time
	cachedRuns    []WorkflowRun
	cachedRunAt   time.Time
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

func (c *client) ListSecurityAlerts(ctx context.Context) ([]SecurityAlert, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedSecurityAlerts(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	alerts, err := c.fetchSecurityAlerts(ctx, token, repo)
	if err != nil {
		return nil, err
	}

	c.storeSecurityAlerts(alerts)
	return alerts, nil
}

func (c *client) ListWorkflowRuns(ctx context.Context) ([]WorkflowRun, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedWorkflowRuns(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	runs, err := c.fetchWorkflowRuns(ctx, token, repo)
	if err != nil {
		return nil, err
	}

	c.storeWorkflowRuns(runs)
	return runs, nil
}

func (c *client) ListWorkflowRunJobs(
	ctx context.Context, runID int64,
) ([]WorkflowJob, error) {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return nil, err
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/repos/%s/actions/runs/%d/jobs", baseURL, repo, runID,
	)

	var wire workflowJobsWire
	if err = c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, err
	}

	jobs := make([]WorkflowJob, 0, len(wire.Jobs))
	for _, w := range wire.Jobs {
		job := WorkflowJob{
			Name:        w.Name,
			Status:      w.Status,
			Conclusion:  w.Conclusion,
			StartedAt:   w.StartedAt,
			CompletedAt: w.CompletedAt,
			DurationMs:  0,
		}
		if w.Status == statusCompleted {
			job.DurationMs = w.CompletedAt.Sub(w.StartedAt).Milliseconds()
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
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

func (c *client) cachedSecurityAlerts() ([]SecurityAlert, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedAlerts != nil && time.Since(c.cachedAlertAt) < cacheTTL {
		return c.cachedAlerts, true
	}
	return nil, false
}

func (c *client) storeSecurityAlerts(alerts []SecurityAlert) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedAlerts = alerts
	c.cachedAlertAt = time.Now()
}

func (c *client) cachedWorkflowRuns() ([]WorkflowRun, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedRuns != nil && time.Since(c.cachedRunAt) < cacheTTL {
		return c.cachedRuns, true
	}
	return nil, false
}

func (c *client) storeWorkflowRuns(runs []WorkflowRun) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedRuns = runs
	c.cachedRunAt = time.Now()
}

// fetchSecurityAlerts lists the repo's open Dependabot, code-scanning, and
// secret-scanning alerts. GitHub Advanced Security features (code scanning,
// secret scanning) return 404 on a repo where they aren't enabled — that's
// treated as "no alerts of that type" rather than an error, since Dependabot
// alerts alone are still a useful degraded result.
func (c *client) fetchSecurityAlerts(
	ctx context.Context, token, repo string,
) ([]SecurityAlert, error) {
	dependabot, err := c.fetchDependabotAlerts(ctx, token, repo)
	if err != nil {
		return nil, err
	}
	codeScanning, err := c.fetchCodeScanningAlerts(ctx, token, repo)
	if err != nil {
		return nil, err
	}
	secretScanning, err := c.fetchSecretScanningAlerts(ctx, token, repo)
	if err != nil {
		return nil, err
	}

	alerts := make(
		[]SecurityAlert,
		0,
		len(dependabot)+len(codeScanning)+len(secretScanning),
	)
	alerts = append(alerts, dependabot...)
	alerts = append(alerts, codeScanning...)
	alerts = append(alerts, secretScanning...)
	return alerts, nil
}

func (c *client) fetchDependabotAlerts(
	ctx context.Context, token, repo string,
) ([]SecurityAlert, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/dependabot/alerts?state=open", baseURL, repo)

	var wires []securityAlertWire
	if err := c.getAllowingNotFound(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	alerts := make([]SecurityAlert, len(wires))
	for i, w := range wires {
		alerts[i] = SecurityAlert{ //nolint:exhaustruct // type-specific fields left zero
			Type:        SecurityAlertTypeDependabot,
			Number:      w.Number,
			PackageName: w.Dependency.Package.Name,
			Ecosystem:   w.Dependency.Package.Ecosystem,
			Severity:    w.SecurityVulnerability.Severity,
			Summary:     w.SecurityAdvisory.Summary,
			URL:         w.HTMLURL,
			CreatedAt:   w.CreatedAt,
		}
	}
	return alerts, nil
}

func (c *client) fetchCodeScanningAlerts(
	ctx context.Context, token, repo string,
) ([]SecurityAlert, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/code-scanning/alerts?state=open",
		baseURL,
		repo,
	)

	var wires []codeScanningAlertWire
	if err := c.getAllowingNotFound(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	alerts := make([]SecurityAlert, len(wires))
	for i, w := range wires {
		alerts[i] = SecurityAlert{ //nolint:exhaustruct // type-specific fields left zero
			Type:      SecurityAlertTypeCodeScanning,
			Number:    w.Number,
			Severity:  w.Rule.SecuritySeverityLevel,
			Summary:   w.Rule.Description,
			URL:       w.HTMLURL,
			CreatedAt: w.CreatedAt,
			RuleID:    w.Rule.ID,
			FilePath:  w.MostRecentInstance.Location.Path,
			Line:      w.MostRecentInstance.Location.StartLine,
		}
	}
	return alerts, nil
}

func (c *client) fetchSecretScanningAlerts(
	ctx context.Context, token, repo string,
) ([]SecurityAlert, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/secret-scanning/alerts?state=open",
		baseURL,
		repo,
	)

	var wires []secretScanningAlertWire
	if err := c.getAllowingNotFound(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	alerts := make([]SecurityAlert, len(wires))
	for i, w := range wires {
		alerts[i] = SecurityAlert{ //nolint:exhaustruct // type-specific fields left zero
			Type:                  SecurityAlertTypeSecretScanning,
			Number:                w.Number,
			URL:                   w.HTMLURL,
			CreatedAt:             w.CreatedAt,
			SecretTypeDisplayName: w.SecretTypeDisplayName,
		}
	}
	return alerts, nil
}

// fetchFailingPullRequests lists the repo's open pull requests and, for each,
// fetches the check runs on its head commit. Only pull requests with at
// least one failing check run, carrying DependenciesLabel, are returned —
// a PR a human or Claude Code session opened already has someone actively
// driving it to green, unlike an unattended Renovate PR.
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
		pr := PullRequest{
			Number:        w.Number,
			Title:         w.Title,
			URL:           w.HTMLURL,
			Author:        w.User.Login,
			UpdatedAt:     w.UpdatedAt,
			HeadSHA:       w.Head.SHA,
			Labels:        labelNames(w.Labels),
			FailingChecks: checks,
		}
		if !pr.HasLabel(DependenciesLabel) {
			continue
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func labelNames(labels []labelWire) []string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names
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
		if run.Status != statusCompleted || !failingConclusions[run.Conclusion] {
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

// runsPerEvent is how many recent runs of each event (pull_request, push) to
// fetch — enough to show a useful recent trend without over-fetching.
const runsPerEvent = 20

// fetchWorkflowRuns lists the repo's most recent pull-request and push
// (main branch) GitHub Actions workflow runs, in two separate requests so
// each kind gets its own recency window instead of one competing for the
// same page.
func (c *client) fetchWorkflowRuns(
	ctx context.Context, token, repo string,
) ([]WorkflowRun, error) {
	prRuns, err := c.fetchWorkflowRunsByEvent(ctx, token, repo, "pull_request")
	if err != nil {
		return nil, err
	}
	pushRuns, err := c.fetchWorkflowRunsByEvent(ctx, token, repo, "push")
	if err != nil {
		return nil, err
	}

	runs := make([]WorkflowRun, 0, len(prRuns)+len(pushRuns))
	runs = append(runs, prRuns...)
	runs = append(runs, pushRuns...)
	return runs, nil
}

func (c *client) fetchWorkflowRunsByEvent(
	ctx context.Context, token, repo, event string,
) ([]WorkflowRun, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/actions/runs?event=%s&per_page=%d",
		baseURL, repo, event, runsPerEvent,
	)

	var wire workflowRunsWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, err
	}

	runs := make([]WorkflowRun, 0, len(wire.WorkflowRuns))
	for _, w := range wire.WorkflowRuns {
		run := WorkflowRun{
			ID:         w.ID,
			Name:       w.Name,
			Event:      w.Event,
			Branch:     w.HeadBranch,
			Status:     w.Status,
			Conclusion: w.Conclusion,
			URL:        w.HTMLURL,
			StartedAt:  w.RunStartedAt,
			DurationMs: 0,
		}
		if w.Status == statusCompleted {
			run.DurationMs = w.UpdatedAt.Sub(w.RunStartedAt).Milliseconds()
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// getAllowingNotFound behaves like get, except a 404 response leaves dst
// untouched (its zero value — an empty slice for the callers above) instead
// of returning an error. GitHub 404s the code-scanning/secret-scanning
// alerts endpoints on a repo where that GHAS feature isn't enabled, which is
// a valid "no alerts of this type" state, not a failure.
func (c *client) getAllowingNotFound(
	ctx context.Context, endpoint, token string, dst any,
) error {
	err := c.get(ctx, endpoint, token, dst)
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
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

		if resp.StatusCode == http.StatusNotFound {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"%w: %s", errNotFound, string(raw),
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

// IsTransientAPIError reports whether err is a known-benign, self-healing
// failure (a timeout) rather than a real bug, so callers polling on an
// interval can log it at a lower level than a persistent failure.
func IsTransientAPIError(err error) bool {
	return isTransientErr(err)
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
