// Package bmc is a client for the Belgian Mobility Company open data gateway
// (Azure APIM) serving SNCB/NMBS GTFS static and GTFS-Realtime feeds. The key
// (BMC_PARTNER_KEY) is sent as the "bmc-partner-key" header.
package bmc

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// operatorSlug is the BMC path segment for SNCB/NMBS.
const operatorSlug = "nmbssncb"

// ErrNotConfigured is returned when no BMC_PARTNER_KEY is set; callers
// degrade gracefully.
var ErrNotConfigured = errors.New("bmc: no partner key configured")

// RateLimitedError is an HTTP 429. Retry-After is the gateway's only backoff
// signal.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("bmc: rate limited, retry after %s", e.RetryAfter)
}

// StaticOptions are the previous import's validators for a conditional GET;
// either may be empty.
type StaticOptions struct {
	ETag         string
	LastModified string
}

// StaticResult is a completed static-feed fetch.
type StaticResult struct {
	// Body is the raw zip. Empty when NotModified is true.
	Body []byte
	// ETag / LastModified are stored for the next conditional GET.
	ETag         string
	LastModified string
	// NotModified is true on a 304.
	NotModified bool
}

// FeedTripUpdate and FeedAlert are the GTFS-Realtime feed path segments.
const (
	FeedTripUpdate = "rt/trip-update"
	FeedAlert      = "rt/alert"
)

// UpstreamError is a non-2xx, non-429 realtime response, so callers can back
// off on a 5xx and surface anything else.
type UpstreamError struct {
	StatusCode int
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("bmc: unexpected status %d", e.StatusCode)
}

// RealtimeResult is a GTFS-Realtime fetch. Body is always protobuf: the
// Content-Type is asserted, since the gateway defaults to JSON.
type RealtimeResult struct {
	Body []byte
}

// UnexpectedContentTypeError is a 200 that's neither protobuf nor JSON,
// e.g. an HTML error page under gateway overload. Transient; callers back off.
type UnexpectedContentTypeError struct {
	Feed        string
	ContentType string
}

func (e *UnexpectedContentTypeError) Error() string {
	return fmt.Sprintf(
		"bmc: expected protobuf response for %s, got content-type %q",
		e.Feed, e.ContentType,
	)
}

// Client fetches feeds from the BMC gateway.
type Client interface {
	// FetchStatic downloads the static zip, honouring opts' validators.
	FetchStatic(ctx context.Context, opts StaticOptions) (*StaticResult, error)
	// FetchRealtime downloads one GTFS-Realtime feed as protobuf.
	FetchRealtime(ctx context.Context, feed string) (*RealtimeResult, error)
}
