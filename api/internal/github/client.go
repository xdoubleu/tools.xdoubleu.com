package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
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

const statusCompleted = "completed"

// errNotFound marks a 404 so getAllowingNotFound can tolerate it.
var errNotFound = errors.New("github: not found")

// errTransientStatus marks a retryable upstream 5xx/429 that exhausted its
// retries; IsTransientAPIError treats it as transient so collectors log it
// at Warn instead of Error→Sentry.
var errTransientStatus = errors.New("github: transient upstream status")

const (
	maxAttempts = 4
	// cacheTTL keeps the dashboard off GitHub's rate limit.
	cacheTTL = 45 * time.Second
)

// configStore resolves the admin-picked repo on every call.
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

// New creates a GitHub client. With no repo picked or no connection, every
// call returns ErrNotConfigured.
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

// DismissSecurityAlert dismisses one alert and clears the security-alerts cache.
func (c *client) DismissSecurityAlert(
	ctx context.Context, alertType SecurityAlertType, alertNumber int64, reason string,
) error {
	repo, err := c.resolveRepo(ctx)
	if err != nil {
		return err
	}

	endpoint, body, err := dismissRequest(repo, alertType, alertNumber, reason)
	if err != nil {
		return err
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return ErrNotConfigured
	}
	if err != nil {
		return err
	}

	if patchErr := c.patch(ctx, endpoint, token, body); patchErr != nil {
		return patchErr
	}

	c.mu.Lock()
	c.cachedAlerts = nil
	c.mu.Unlock()
	return nil
}

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

// fetchSecurityAlerts lists open Dependabot, code-scanning and secret-scanning
// alerts. GHAS endpoints 404 when disabled; that means "none", not an error.
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
		patched := ""
		if fp := w.SecurityVulnerability.FirstPatchedVersion; fp != nil {
			patched = fp.Identifier
		}
		alerts[i] = SecurityAlert{ //nolint:exhaustruct // type-specific fields left zero
			Type:                SecurityAlertTypeDependabot,
			Number:              w.Number,
			PackageName:         w.Dependency.Package.Name,
			Ecosystem:           w.Dependency.Package.Ecosystem,
			Severity:            w.SecurityVulnerability.Severity,
			Summary:             w.SecurityAdvisory.Summary,
			AdvisoryID:          w.SecurityAdvisory.GHSAID,
			FirstPatchedVersion: patched,
			URL:                 w.HTMLURL,
			CreatedAt:           w.CreatedAt,
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

const runsPerEvent = 20

// fetchWorkflowRuns fetches PR and push runs separately so each gets its own
// recency window, then merges them newest-first by start time.
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
	// Newest first, so the order doesn't depend on which event was fetched first.
	slices.SortStableFunc(runs, func(a, b WorkflowRun) int {
		return b.StartedAt.Compare(a.StartedAt)
	})
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

// getAllowingNotFound is get, but a 404 leaves dst at its zero value.
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
				"%w: github API returned %d: %s",
				errTransientStatus, resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode == http.StatusNotFound {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"%w: %s", errNotFound, string(raw),
			)
		}

		if resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"%w (%d): %s", ErrAccessDenied, resp.StatusCode, string(raw),
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

func (c *client) patch(ctx context.Context, endpoint, token, body string) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodPatch, endpoint, strings.NewReader(body),
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
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
				"%w: github API returned %d: %s",
				errTransientStatus, resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"github API returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		return false, nil
	})
}

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

// SetBaseURL overrides the GitHub API base URL (tests only).
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the retry backoff base (tests only).
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

// IsTransientAPIError reports whether err is self-healing: a timeout, a DNS
// lookup failure, or a retryable upstream 5xx/429 that exhausted its retries.
func IsTransientAPIError(err error) bool {
	return errors.Is(err, errTransientStatus) || isTransientErr(err)
}

func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return true
	}
	// DNS lookup failures (e.g. a flaky Docker resolver) are transient.
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}
