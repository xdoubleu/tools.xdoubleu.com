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
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.digitalocean.com"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 15 * time.Second

const (
	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
	// cacheTTL is how long a fetched deployment is served from memory before
	// the next call re-fetches.
	cacheTTL = 45 * time.Second
)

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	token      string
	appID      string

	mu       sync.Mutex
	cached   *Deployment
	cachedAt time.Time
}

// New creates a DigitalOcean App Platform client. token is a DO access token
// and appID identifies the app. When either is empty the client is considered
// not configured and every call returns ErrNotConfigured.
func New(logger *slog.Logger, token, appID string) Client {
	return &client{ //nolint:exhaustruct // cache fields start zero-valued
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
		token: token,
		appID: appID,
	}
}

func (c *client) LatestDeployment(ctx context.Context) (*Deployment, error) {
	if c.token == "" || c.appID == "" {
		return nil, ErrNotConfigured
	}

	if cached, ok := c.cachedDeployment(); ok {
		return cached, nil
	}

	deployment, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}

	c.store(deployment)
	return deployment, nil
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

func (c *client) fetch(ctx context.Context) (*Deployment, error) {
	endpoint := fmt.Sprintf("%s/v2/apps/%s/deployments", baseURL, c.appID)

	var wire deploymentsWire
	if err := c.get(ctx, endpoint, &wire); err != nil {
		return nil, err
	}

	if len(wire.Deployments) == 0 {
		return nil, nil //nolint:nilnil // no deployment yet is a valid state
	}

	latest := wire.Deployments[0].toDeployment()
	return &latest, nil
}

func (c *client) get(ctx context.Context, endpoint string, dst any) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodGet, endpoint, nil,
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.token)

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
			return false, fmt.Errorf(
				"digitalocean API returned %d: %s",
				resp.StatusCode, string(raw),
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
