package bmc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	partnerKeyHeader = "bmc-partner-key"
	requestTimeout   = 60 * time.Second
	// maxStaticBytes caps the download (~9 MB real zip).
	maxStaticBytes = 64 << 20
)

// scheme is https in production; a test lowers it to http for httptest.
//
//nolint:gochecknoglobals //test seam, see client_internal_test.go
var scheme = "https"

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	host       string
	partnerKey string
}

// New builds a BMC client; an empty partnerKey makes FetchStatic return
// ErrNotConfigured.
func New(logger *slog.Logger, host, partnerKey string) Client {
	return &client{
		logger:     logger,
		httpClient: &http.Client{Timeout: requestTimeout},
		host:       host,
		partnerKey: partnerKey,
	}
}

func (c *client) FetchStatic(
	ctx context.Context,
	opts StaticOptions,
) (*StaticResult, error) {
	if c.partnerKey == "" {
		return nil, ErrNotConfigured
	}

	endpoint := fmt.Sprintf(
		"%s://%s/api/gtfs/feed/%s/static", scheme, c.host, operatorSlug,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(partnerKeyHeader, c.partnerKey)
	if opts.ETag != "" {
		req.Header.Set("If-None-Match", opts.ETag)
	}
	if opts.LastModified != "" {
		req.Header.Set("If-Modified-Since", opts.LastModified)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return &StaticResult{NotModified: true}, nil //nolint:exhaustruct //304
	case http.StatusOK:
		// handled below
	case http.StatusTooManyRequests:
		return nil, &RateLimitedError{RetryAfter: parseRetryAfter(resp)}
	default:
		return nil, fmt.Errorf("bmc: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStaticBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxStaticBytes {
		return nil, fmt.Errorf(
			"bmc: static feed exceeds %d bytes",
			int64(maxStaticBytes),
		)
	}

	return &StaticResult{
		Body:         body,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		NotModified:  false,
	}, nil
}

// parseRetryAfter reads Retry-After as delta-seconds, defaulting to 30s.
func parseRetryAfter(resp *http.Response) time.Duration {
	const fallback = 30 * time.Second
	raw := resp.Header.Get("Retry-After")
	if raw == "" {
		return fallback
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		return fallback
	}
	return time.Duration(secs) * time.Second
}
