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
	"sync"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://sentry.io"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 15 * time.Second

const (
	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
	// cacheTTL is how long a fetched issue list is served from memory before
	// the next call re-fetches.
	cacheTTL = 45 * time.Second
)

// configStore is the subset of *repositories.OAuthConnectionsRepository used
// to resolve the admin-picked org/projects fresh on every call, instead of
// static values baked in at boot (mirrors oauthconn's own narrow
// connectionStore).
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
}

// New creates a Sentry API client. tokenFn resolves a live OAuth bearer
// token (see internal/oauthconn) and configRepo resolves the admin-picked
// org/projects on every call. When no org/projects are picked, or tokenFn
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

// resolveConfig reads the admin-picked org/projects from the stored
// connection config. Returns ErrNotConfigured when the provider isn't
// connected or no org/projects have been picked yet.
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

// fetchAll fetches unresolved issues for every configured project
// sequentially (N is small and results are cache-backed for cacheTTL, so no
// added concurrency), tags each with its project, and merges them into one
// list sorted by LastSeen descending.
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
			return true, fmt.Errorf(
				"sentry API returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"sentry API returned %d: %s", resp.StatusCode, string(raw),
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

// SetBaseURL overrides the Sentry API base URL. Intended for tests only.
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
