package sentryapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://sentry.io"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

// apiError is a non-2xx response, structured so 5xx can be detected via
// errors.As.
type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("sentry API returned %d: %s", e.status, e.body)
}

const apiTimeout = 15 * time.Second

const (
	maxAttempts = 4
	cacheTTL    = 45 * time.Second
	// transactionStatsPerPage is above the real route count; no pagination needed.
	transactionStatsPerPage = 100
)

// configStore resolves the admin-picked org/projects on every call.
type configStore interface {
	Get(
		ctx context.Context, provider models.OAuthProvider,
	) (*oauth2.Token, *models.OAuthConnection, error)
}

type projectsConfig struct {
	Org      string   `json:"org"`
	Projects []string `json:"projects"`
}

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	tokenFn    oauthconn.TokenFunc
	configRepo configStore

	mu       sync.Mutex
	cached   []Issue
	cachedAt time.Time

	statsMu       sync.Mutex
	cachedStats   []TransactionStat
	cachedStatsAt time.Time
}

// New creates a Sentry client. With no org/projects picked or no connection,
// every call returns ErrNotConfigured.
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

func (c *client) ListUnresolvedIssues(ctx context.Context) ([]Issue, error) {
	cfg, err := c.resolveConfig(ctx)
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

	issues, err := c.fetchAll(ctx, token, cfg)
	if err != nil {
		return nil, err
	}

	c.store(issues)
	return issues, nil
}

func (c *client) ListTransactionStats(ctx context.Context) ([]TransactionStat, error) {
	cfg, err := c.resolveConfig(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedTransactionStats(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	stats, err := c.fetchTransactionStats(ctx, token, cfg)
	if err != nil {
		return nil, err
	}

	c.storeTransactionStats(stats)
	return stats, nil
}

// ResolveIssue resolves issueID. The endpoint needs no org, but config is
// checked first so an unconfigured connection fails consistently.
func (c *client) ResolveIssue(ctx context.Context, issueID string) error {
	if _, err := c.resolveConfig(ctx); err != nil {
		return err
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		// Config exists, so a refusal here means a stale scope.
		return ErrReauthRequired
	}
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/0/issues/%s/", baseURL, issueID)
	if putErr := c.put(ctx, endpoint, token, resolveIssueBody); putErr != nil {
		return putErr
	}

	// Invalidate the cached issue list.
	c.mu.Lock()
	c.cached = nil
	c.mu.Unlock()
	return nil
}

func (c *client) resolveConfig(ctx context.Context) (projectsConfig, error) {
	_, conn, err := c.configRepo.Get(ctx, models.OAuthProviderSentry)
	if errors.Is(err, database.ErrResourceNotFound) {
		return projectsConfig{}, ErrNotConfigured
	}
	if err != nil {
		return projectsConfig{}, err
	}
	if len(conn.Config) == 0 {
		return projectsConfig{}, ErrNotConfigured
	}

	var cfg projectsConfig
	if unmarshalErr := json.Unmarshal(conn.Config, &cfg); unmarshalErr != nil {
		return projectsConfig{}, unmarshalErr
	}
	if cfg.Org == "" || len(cfg.Projects) == 0 {
		return projectsConfig{}, ErrNotConfigured
	}
	return cfg, nil
}

// fetchAll fetches each project's unresolved issues sequentially, tags them,
// and sorts by LastSeen descending.
func (c *client) fetchAll(
	ctx context.Context, token string, cfg projectsConfig,
) ([]Issue, error) {
	var all []Issue
	for _, project := range cfg.Projects {
		issues, err := c.fetch(ctx, token, cfg.Org, project)
		if err != nil {
			return nil, err
		}
		for i := range issues {
			issues[i].Project = project
		}
		all = append(all, issues...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].LastSeen.After(all[j].LastSeen)
	})
	return all, nil
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

func (c *client) cachedTransactionStats() ([]TransactionStat, bool) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	if c.cachedStats != nil && time.Since(c.cachedStatsAt) < cacheTTL {
		return c.cachedStats, true
	}
	return nil, false
}

func (c *client) storeTransactionStats(stats []TransactionStat) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.cachedStats = stats
	c.cachedStatsAt = time.Now()
}

type eventsResponse struct {
	Data []transactionStatWire `json:"data"`
}

// fetchTransactionStats queries the org-level Discover API for 24h p95 and
// count per transaction in one call, keeping only the picked projects.
func (c *client) fetchTransactionStats(
	ctx context.Context, token string, cfg projectsConfig,
) ([]TransactionStat, error) {
	endpoint := fmt.Sprintf(
		"%s/api/0/organizations/%s/events/?%s",
		baseURL, cfg.Org, transactionStatsQuery(),
	)

	var resp eventsResponse
	if err := c.get(ctx, endpoint, token, &resp); err != nil {
		return nil, err
	}

	allowed := make(map[string]bool, len(cfg.Projects))
	for _, p := range cfg.Projects {
		allowed[p] = true
	}

	stats := make([]TransactionStat, 0, len(resp.Data))
	for _, w := range resp.Data {
		if !allowed[w.Project] {
			continue
		}
		stats = append(stats, w.toTransactionStat())
	}
	return stats, nil
}

// transactionStatsQuery builds the Discover query over the spans dataset (the
// transactions dataset is deprecated).
func transactionStatsQuery() string {
	params := url.Values{
		"dataset": {"spans"},
		"field": {
			"transaction",
			"project",
			"p95(span.duration)",
			"count()",
		},
		"query":       {"is_transaction:true"},
		"sort":        {"-p95(span.duration)"},
		"statsPeriod": {"24h"},
		"per_page":    {strconv.Itoa(transactionStatsPerPage)},
	}
	return params.Encode()
}

func (c *client) fetch(
	ctx context.Context, token, org, project string,
) ([]Issue, error) {
	endpoint := fmt.Sprintf(
		"%s/api/0/projects/%s/%s/issues/?query=%s",
		baseURL, org, project, url.QueryEscape("is:unresolved"),
	)

	var wires []issueWire
	if err := c.get(ctx, endpoint, token, &wires); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(wires))
	for _, w := range wires {
		issues = append(issues, w.toIssue())
	}
	return issues, nil
}

const resolveIssueBody = `{"status":"resolved"}`

func (c *client) get(ctx context.Context, endpoint, token string, dst any) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodGet, endpoint, nil,
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, &apiError{status: resp.StatusCode, body: string(raw)}
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, &apiError{status: resp.StatusCode, body: string(raw)}
		}

		return false, json.NewDecoder(resp.Body).Decode(dst)
	})
}

func (c *client) put(ctx context.Context, endpoint, token, body string) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodPut, endpoint, strings.NewReader(body),
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, &apiError{status: resp.StatusCode, body: string(raw)}
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, &apiError{status: resp.StatusCode, body: string(raw)}
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
		c.logger.DebugContext(ctx, "retrying sentry request",
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

// SetBaseURL overrides the Sentry API base URL (tests only).
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

// IsTransientAPIError reports whether err is a self-healing 5xx or timeout.
func IsTransientAPIError(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.status >= http.StatusInternalServerError {
		return true
	}
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
