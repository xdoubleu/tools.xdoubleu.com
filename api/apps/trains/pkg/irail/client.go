// Package irail is a small client for the community-run iRail API
// (https://docs.irail.be). It rate-limits below iRail's 3 req/s per-IP limit,
// retries a 429 once, caches responses briefly, and keeps a short history of
// failures for diagnostics.
package irail

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
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DefaultBaseURL is iRail's versioned API root.
const DefaultBaseURL = "https://api.irail.be/v1"

const (
	paramLang       = "lang"
	requestTimeout  = 15 * time.Second
	cacheTTL        = 20 * time.Second
	maxRetryAfter   = 3 * time.Second
	requestInterval = 400 * time.Millisecond
	requestBurst    = 3
	maxErrorHistory = 20
	maxBodyBytes    = 8 << 20
)

// ErrRateLimited is returned when iRail answers 429 twice in a row.
var ErrRateLimited = errors.New("irail: rate limited")

// ErrNotFound is returned when iRail has no such train (HTTP 404).
var ErrNotFound = errors.New("irail: not found")

// Client is the slice of iRail the trains app uses.
type Client interface {
	// Stations lists every station with names in lang ("nl", "fr" or "en").
	Stations(ctx context.Context, lang string) ([]Station, error)
	// Connections plans journeys around when, with live delays.
	Connections(
		ctx context.Context, from, to string, when time.Time, arriveBy bool,
	) ([]Connection, error)
	// Vehicle returns one train's stops with live state on a service date.
	Vehicle(ctx context.Context, vehicleID string, date time.Time) (*Vehicle, error)
	// RawVehicle returns iRail's unparsed response for one train.
	RawVehicle(ctx context.Context, vehicleID string, date time.Time) ([]byte, error)
	// Diagnostics reports recent request failures.
	Diagnostics() Diagnostics
}

// Failure is one failed iRail request.
type Failure struct {
	At       time.Time `json:"at"`
	Endpoint string    `json:"endpoint"`
	Status   int       `json:"status"`
	Message  string    `json:"message"`
}

// Diagnostics counts requests and keeps the most recent failures.
type Diagnostics struct {
	Requests    int       `json:"requests"`
	RateLimited int       `json:"rate_limited"`
	Failures    int       `json:"failures"`
	Recent      []Failure `json:"recent"`
}

// HTTPClient is the live Client.
type HTTPClient struct {
	logger    *slog.Logger
	http      *http.Client
	baseURL   string
	userAgent string
	limiter   *rate.Limiter

	mu    sync.Mutex
	cache map[string]cacheEntry
	diag  Diagnostics
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

var _ Client = (*HTTPClient)(nil)

// New builds a client. iRail asks for a User-Agent that identifies the app and
// a way to reach its owner.
func New(logger *slog.Logger, baseURL, userAgent string) *HTTPClient {
	//nolint:exhaustruct //diag starts zeroed
	return &HTTPClient{
		logger: logger,
		// The URL is fixed, not user-supplied, so no safedial.
		http:      &http.Client{Timeout: requestTimeout},
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: userAgent,
		limiter:   rate.NewLimiter(rate.Every(requestInterval), requestBurst),
		cache:     make(map[string]cacheEntry),
	}
}

func (c *HTTPClient) Stations(ctx context.Context, lang string) ([]Station, error) {
	body, err := c.get(ctx, "stations", url.Values{paramLang: {lang}}, 0)
	if err != nil {
		return nil, err
	}
	return parseStations(body)
}

func (c *HTTPClient) Connections(
	ctx context.Context, from, to string, when time.Time, arriveBy bool,
) ([]Connection, error) {
	local := when.In(brussels)
	timesel := "departure"
	if arriveBy {
		timesel = "arrival"
	}
	q := url.Values{
		"from":    {from},
		"to":      {to},
		"date":    {local.Format("020106")},
		"time":    {local.Format("1504")},
		"timesel": {timesel},
		paramLang: {"en"},
		"results": {"6"},
	}
	body, err := c.get(ctx, "connections", q, cacheTTL)
	if err != nil {
		return nil, err
	}
	return parseConnections(body)
}

func (c *HTTPClient) Vehicle(
	ctx context.Context, vehicleID string, date time.Time,
) (*Vehicle, error) {
	body, err := c.RawVehicle(ctx, vehicleID, date)
	if err != nil {
		return nil, err
	}
	return parseVehicle(body)
}

func (c *HTTPClient) RawVehicle(
	ctx context.Context, vehicleID string, date time.Time,
) ([]byte, error) {
	q := url.Values{
		"id":      {vehicleID},
		"date":    {date.In(brussels).Format("020106")},
		paramLang: {"en"},
	}
	return c.get(ctx, "vehicle", q, cacheTTL)
}

func (c *HTTPClient) Diagnostics() Diagnostics {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.diag
	out.Recent = append([]Failure(nil), c.diag.Recent...)
	return out
}

func (c *HTTPClient) get(
	ctx context.Context, endpoint string, q url.Values, ttl time.Duration,
) ([]byte, error) {
	q.Set("format", "json")
	key := endpoint + "?" + q.Encode()
	if ttl > 0 {
		if body, ok := c.cached(key); ok {
			return body, nil
		}
	}

	body, err := c.fetch(ctx, endpoint, key)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		c.mu.Lock()
		c.cache[key] = cacheEntry{body: body, expires: time.Now().Add(ttl)}
		c.evictExpiredLocked()
		c.mu.Unlock()
	}
	return body, nil
}

func (c *HTTPClient) cached(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.body, true
}

func (c *HTTPClient) evictExpiredLocked() {
	now := time.Now()
	for k, e := range c.cache {
		if now.After(e.expires) {
			delete(c.cache, k)
		}
	}
}

// fetch performs one request, retrying a 429 once after Retry-After.
func (c *HTTPClient) fetch(ctx context.Context, endpoint, key string) ([]byte, error) {
	for attempt := range 2 {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		body, status, retryAfter, err := c.do(ctx, key)
		c.record(endpoint, status, err)
		switch {
		case status == http.StatusTooManyRequests && attempt == 0:
			if waitErr := sleep(ctx, retryAfter); waitErr != nil {
				return nil, waitErr
			}
			continue
		case status == http.StatusTooManyRequests:
			return nil, ErrRateLimited
		case status == http.StatusNotFound:
			return nil, ErrNotFound
		case err != nil:
			return nil, err
		}
		return body, nil
	}
	return nil, ErrRateLimited
}

func (c *HTTPClient) do(
	ctx context.Context, key string,
) ([]byte, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL+"/"+key, nil,
	)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, resp.StatusCode, retryAfter, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, retryAfter, fmt.Errorf(
			"irail: unexpected status %d", resp.StatusCode,
		)
	}
	return body, resp.StatusCode, retryAfter, nil
}

func (c *HTTPClient) record(endpoint string, status int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diag.Requests++
	if status == http.StatusTooManyRequests {
		c.diag.RateLimited++
	}
	if err == nil {
		return
	}
	c.diag.Failures++
	c.diag.Recent = append(c.diag.Recent, Failure{
		At: time.Now(), Endpoint: endpoint, Status: status, Message: err.Error(),
	})
	if len(c.diag.Recent) > maxErrorHistory {
		c.diag.Recent = c.diag.Recent[len(c.diag.Recent)-maxErrorHistory:]
	}
	c.logger.Warn("trains: irail request failed",
		slog.String("endpoint", endpoint), slog.Int("status", status),
		slog.Any("error", err))
}

func parseRetryAfter(h string) time.Duration {
	secs, err := strconv.Atoi(h)
	if err != nil || secs < 0 {
		return time.Second
	}
	return min(time.Duration(secs)*time.Second, maxRetryAfter)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// decode unmarshals body into v.
func decode(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("irail: decode response: %w", err)
	}
	return nil
}
