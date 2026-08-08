package digitalocean

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
var baseURL = "https://api.digitalocean.com"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

// apiError is a non-2xx response from the DigitalOcean API, kept structured
// so callers can distinguish specific known-benign error bodies (e.g. a log
// phase that never ran) from real failures via errors.As.
type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("digitalocean API returned %d: %s", e.status, e.body)
}

const apiTimeout = 15 * time.Second

const (
	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
	// cacheTTL is how long a fetched deployment is served from memory before
	// the next call re-fetches.
	cacheTTL = 45 * time.Second
)

// configStore is the subset of *repositories.OAuthConnectionsRepository used
// to resolve the admin-picked app ID fresh on every call, instead of a
// static value baked in at boot (mirrors oauthconn's own narrow
// connectionStore).
type configStore interface {
	Get(
		ctx context.Context, provider models.OAuthProvider,
	) (*oauth2.Token, *models.OAuthConnection, error)
}

type appConfig struct {
	AppID string `json:"app_id"`
}

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	wsClient   *http.Client
	tokenFn    oauthconn.TokenFunc
	configRepo configStore

	mu       sync.Mutex
	cached   *Deployment
	cachedAt time.Time
}

// New creates a DigitalOcean App Platform client. tokenFn resolves a live
// OAuth bearer token (see internal/oauthconn) and configRepo resolves the
// admin-picked app ID on every call. When no app ID is picked, or tokenFn
// reports the provider isn't connected, every call returns ErrNotConfigured.
func New(
	logger *slog.Logger, tokenFn oauthconn.TokenFunc, configRepo configStore,
) Client {
	return &client{ //nolint:exhaustruct // cache fields start zero-valued
		logger:     logger,
		httpClient: &http.Client{Timeout: apiTimeout},
		// wsClient deliberately has no Timeout: coder/websocket turns one
		// into a deadline on the whole connection, and the live log read
		// bounds itself with liveLogDeadline instead.
		wsClient:   &http.Client{},
		tokenFn:    tokenFn,
		configRepo: configRepo,
	}
}

func (c *client) LatestDeployment(ctx context.Context) (*Deployment, error) {
	appID, err := c.resolveAppID(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedDeployment(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	deployment, err := c.fetch(ctx, token, appID)
	if err != nil {
		return nil, err
	}

	c.store(deployment)
	return deployment, nil
}

// resolveAppID reads the admin-picked app ID from the stored connection
// config. Returns ErrNotConfigured when the provider isn't connected or no
// app has been picked yet.
func (c *client) resolveAppID(ctx context.Context) (string, error) {
	_, conn, err := c.configRepo.Get(ctx, models.OAuthProviderDigitalOcean)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", ErrNotConfigured
	}
	if err != nil {
		return "", err
	}
	if len(conn.Config) == 0 {
		return "", ErrNotConfigured
	}

	var cfg appConfig
	if unmarshalErr := json.Unmarshal(conn.Config, &cfg); unmarshalErr != nil {
		return "", unmarshalErr
	}
	if cfg.AppID == "" {
		return "", ErrNotConfigured
	}
	return cfg.AppID, nil
}

func (c *client) cachedDeployment() (*Deployment, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cachedAt.IsZero() && time.Since(c.cachedAt) < cacheTTL {
		return c.cached, true
	}
	return nil, false
}

func (c *client) store(deployment *Deployment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = deployment
	c.cachedAt = time.Now()
}

func (c *client) fetch(ctx context.Context, token, appID string) (*Deployment, error) {
	endpoint := fmt.Sprintf("%s/v2/apps/%s/deployments", baseURL, appID)

	var wire deploymentsWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, err
	}

	if len(wire.Deployments) == 0 {
		return nil, nil //nolint:nilnil // no deployment yet is a valid state
	}

	latest := wire.Deployments[0].toDeployment()
	return &latest, nil
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
				"digitalocean API returned %d: %s",
				resp.StatusCode, string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, &apiError{status: resp.StatusCode, body: string(raw)}
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
		c.logger.DebugContext(ctx, "retrying digitalocean request",
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

// SetBaseURL overrides the DigitalOcean API base URL. Intended for tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the exponential-backoff base delay. Intended for
// tests only so retry tests run without real wall-clock sleeps.
func SetBackoffBase(d time.Duration) { backoffBase = d }

// SetLiveLogDeadline overrides the per-component live-log read deadline and
// returns the previous value so the caller can restore it. Intended for tests
// only, so the never-closing-socket path can be driven without a multi-second
// wait.
func SetLiveLogDeadline(d time.Duration) time.Duration {
	prev := liveLogDeadline
	liveLogDeadline = d
	return prev
}

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
// failure (a 401 — DO-edge wording for an mTLS blip, not our own cert config
// — or a timeout) rather than a real bug, so callers polling on an interval
// can log it at a lower level than a persistent failure.
func IsTransientAPIError(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.status == http.StatusUnauthorized {
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
