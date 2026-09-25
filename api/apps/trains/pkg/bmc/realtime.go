package bmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxRealtimeBytes caps a realtime download (~124 KB observed).
const maxRealtimeBytes = 8 << 20

func (c *client) FetchRealtime(
	ctx context.Context,
	feed string,
) (*RealtimeResult, error) {
	if c.partnerKey == "" {
		return nil, ErrNotConfigured
	}

	endpoint := fmt.Sprintf(
		"%s://%s/api/gtfs/feed/%s/%s?format=protobuf",
		scheme, c.host, operatorSlug, feed,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(partnerKeyHeader, c.partnerKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusTooManyRequests:
		return nil, &RateLimitedError{RetryAfter: parseRetryAfter(resp)}
	default:
		return nil, &UpstreamError{StatusCode: resp.StatusCode}
	}

	// Assert protobuf: the gateway defaults to JSON (whose int64s corrupt a naive
	// parse) and serves HTML error pages with 200 under overload.
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "protobuf") {
		return nil, &UnexpectedContentTypeError{Feed: feed, ContentType: contentType}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRealtimeBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxRealtimeBytes {
		return nil, fmt.Errorf(
			"bmc: realtime feed %s exceeds %d bytes", feed, int64(maxRealtimeBytes),
		)
	}

	return &RealtimeResult{Body: body}, nil
}
