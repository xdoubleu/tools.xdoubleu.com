// Package tmdb is a minimal client for The Movie Database v3 API.
package tmdb

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
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
	// ListRegionProviders returns the providers TMDB lists for Belgium for
	// movies and series together, by display priority.
	ListRegionProviders(ctx context.Context) ([]RegionProvider, error)
}

type client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
	token      string
}

// New creates a client authenticating with an API Read Access Token.
func New(token string) Client {
	return client{
		httpClient: &http.Client{Timeout: apiTimeout},
		limiter:    rate.NewLimiter(requestsPerSecond, burst),
		token:      token,
	}
}

// withProviders returns the details together with the watch providers.
func withProviders() url.Values {
	return url.Values{"append_to_response": {"watch/providers"}}
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
	err := c.get(ctx, "/movie/"+strconv.FormatInt(id, 10), withProviders(), &resp)
	if err != nil {
		return nil, err
	}
	t := resp.toTitle()
	return &t, nil
}

func (c client) GetSeries(ctx context.Context, id int64) (*Title, error) {
	var resp seriesResponse
	err := c.get(ctx, "/tv/"+strconv.FormatInt(id, 10), withProviders(), &resp)
	if err != nil {
		return nil, err
	}
	t := resp.toTitle()
	return &t, nil
}

func (c client) ListRegionProviders(ctx context.Context) ([]RegionProvider, error) {
	merged := map[int64]priorityProvider{}
	for _, kind := range []string{"movie", "tv"} {
		var list regionProviderList
		err := c.get(ctx, "/watch/providers/"+kind,
			url.Values{"watch_region": {watchRegion}}, &list)
		if err != nil {
			return nil, err
		}
		list.merge(merged)
	}

	all := make([]priorityProvider, 0, len(merged))
	for _, p := range merged {
		all = append(all, p)
	}
	slices.SortFunc(all, func(a, b priorityProvider) int {
		return cmp.Or(cmp.Compare(a.priority, b.priority), cmp.Compare(a.ID, b.ID))
	})
	out := make([]RegionProvider, len(all))
	for i, p := range all {
		out[i] = p.RegionProvider
	}
	return out, nil
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
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDelay(i - 1)):
			}
		}
		retryable, err := c.do(ctx, endpoint, dst)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable {
			break
		}
	}
	return lastErr
}

// backoffDelay doubles from backoffBase with each retry.
func backoffDelay(retry int) time.Duration {
	return backoffBase << retry
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
