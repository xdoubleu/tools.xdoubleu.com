package anthropicadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.anthropic.com"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

// anthropicVersion is the API version header every Anthropic API call
// requires, pinned the same way it would be for a real Messages API call
// elsewhere in this codebase, so a future breaking version bump is a
// deliberate, visible change here.
const anthropicVersion = "2023-06-01"

const apiTimeout = 15 * time.Second

// maxAttempts is the total number of tries for a retryable request.
const maxAttempts = 4

// maxPages bounds how many cursor pages GetClaudeCodeUsage follows for a
// single day, guarding against an API bug (e.g. has_more permanently true)
// turning one job run into an unbounded loop. One org's single-day usage
// is expected to fit in a handful of pages at most.
const maxPages = 50

// apiError is a non-2xx response from the Admin API, kept structured so
// callers can distinguish known-transient statuses (5xx) from real failures.
type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("anthropic admin API returned %d: %s", e.status, e.body)
}

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	apiKey     string
}

// New creates an Anthropic Admin API client. An empty apiKey makes every
// call return ErrNotConfigured.
func New(logger *slog.Logger, apiKey string) Client {
	return &client{
		logger:     logger,
		httpClient: &http.Client{Timeout: apiTimeout},
		apiKey:     apiKey,
	}
}

func (c *client) GetClaudeCodeUsage(
	ctx context.Context, date string,
) ([]UsageRecord, error) {
	if c.apiKey == "" {
		return nil, ErrNotConfigured
	}

	var all []UsageRecord
	page := ""
	for range maxPages {
		wire, err := c.fetchPage(ctx, date, page)
		if err != nil {
			return nil, err
		}
		all = append(all, wire.Data...)

		if !wire.HasMore || wire.NextPage == nil || *wire.NextPage == "" {
			return all, nil
		}
		page = *wire.NextPage
	}
	return all, nil
}

func (c *client) fetchPage(
	ctx context.Context, date, page string,
) (usageReportWire, error) {
	params := url.Values{"starting_at": {date}}
	if page != "" {
		params.Set("page", page)
	}
	endpoint := fmt.Sprintf(
		"%s/v1/organizations/usage_report/claude_code?%s",
		baseURL, params.Encode(),
	)

	var wire usageReportWire
	if err := c.get(ctx, endpoint, &wire); err != nil {
		return usageReportWire{}, err
	}
	return wire, nil
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
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("X-Api-Key", c.apiKey)

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
		c.logger.DebugContext(ctx, "retrying anthropic admin API request",
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

// SetBaseURL overrides the Anthropic Admin API base URL. Intended for tests
// only.
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
// failure (a 5xx or a timeout) rather than a real bug, so callers polling on
// an interval can log it at a lower level than a persistent failure.
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
