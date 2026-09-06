package bmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxRealtimeBytes caps a realtime download. The observed protobuf snapshot
// is ~124 KB (issue #1389); this is generous headroom without letting a
// misbehaving gateway stream unbounded into memory.
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

	// The gateway is documented (issue #1389) to serve JSON by default, with
	// int64s that would silently corrupt a naive delay parse — fail loudly
	// rather than parsing whatever came back if protobuf wasn't honoured.
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "protobuf") {
		return nil, fmt.Errorf(
			"bmc: expected protobuf response for %s, got content-type %q",
			feed, contentType,
		)
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
