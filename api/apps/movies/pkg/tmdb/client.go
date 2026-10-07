// Package tmdb is a minimal client for The Movie Database v3 API.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.themoviedb.org/3"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

const (
	// All attempts plus backoff (~24s) stay under kamal-proxy's 30s default
	// response timeout; callers are request handlers.
	apiTimeout = 5 * time.Second
	language   = "en-US"

	// TMDB allows roughly 50 req/s; stay well below it.
	requestsPerSecond = 20
	burst             = 10

	maxAttempts = 4

	// maxErrorBody caps how much of an error response is quoted.
	maxErrorBody = 1024
)

// Client is the subset of TMDB used for the backlog.
type Client interface {
	// Search returns movies and series matching query; people are dropped.
	Search(ctx context.Context, query string) ([]Title, error)
	// GetMovie returns ErrNotFound for an unknown ID.
	GetMovie(ctx context.Context, id int64) (*Title, error)
	// GetSeries returns ErrNotFound for an unknown ID.
	GetSeries(ctx context.Context, id int64) (*Title, error)
}

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	limiter    *rate.Limiter
	token      string
}

// New creates a client authenticating with an API Read Access Token.
func New(logger *slog.Logger, token string) Client {
	return client{
		logger:     logger,
		httpClient: &http.Client{Timeout: apiTimeout},
		limiter:    rate.NewLimiter(requestsPerSecond, burst),
		token:      token,
	}
}

func (c client) Search(ctx context.Context, query string) ([]Title, error) {
	var resp searchResponse
	err := c.get(ctx, "/search/multi", url.Values{
		"query":         {query},
		"include_adult": {"false"},
		"page":          {"1"},
	}, &resp)
	if err != nil {
		return nil, err
	}

	titles := make([]Title, 0, len(resp.Results))
	for _, r := range resp.Results {
		if t, ok := r.toTitle(); ok {
			titles = append(titles, t)
		}
	}
	return titles, nil
}

func (c client) GetMovie(ctx context.Context, id int64) (*Title, error) {
	var resp movieResponse
	if err := c.get(ctx, "/movie/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}
	t := resp.toTitle()
	return &t, nil
}

func (c client) GetSeries(ctx context.Context, id int64) (*Title, error) {
	var resp seriesResponse
	if err := c.get(ctx, "/tv/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}
	t := resp.toTitle()
	return &t, nil
}

func (c client) get(
	ctx context.Context,
	path string,
	params url.Values,
	dst any,
) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("language", language)
	endpoint := baseURL + path + "?" + params.Encode()

	var lastErr error
	for i := range maxAttempts {
		retryable, err := c.do(ctx, endpoint, dst)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable || i == maxAttempts-1 {
			break
		}

		delay := backoffBase * (1 << i)
		c.logger.DebugContext(ctx, "retrying tmdb request",
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

func (c client) do(ctx context.Context, endpoint string, dst any) (bool, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return isTransientErr(err), err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode >= http.StatusInternalServerError:
		return true, statusErr(resp)
	case resp.StatusCode != http.StatusOK:
		return false, statusErr(resp)
	}

	return false, json.NewDecoder(resp.Body).Decode(dst)
}

func statusErr(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return fmt.Errorf("tmdb API returned %d: %s", resp.StatusCode, raw)
}

func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) && urlErr.Timeout()
}

// SetBaseURL overrides the API root. Tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the retry backoff base. Tests only.
func SetBackoffBase(d time.Duration) { backoffBase = d }
