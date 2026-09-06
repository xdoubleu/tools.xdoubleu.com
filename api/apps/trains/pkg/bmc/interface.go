// Package bmc is a thin client for the Belgian Mobility Company (BMC) open
// data portal, an Azure APIM gateway fronting GTFS static + GTFS-Realtime
// feeds per operator. This slice (issue #1390) only needs the SNCB/NMBS
// static timetable; the realtime feeds are added by issue #1393.
//
// The gateway host and subscription key come from internal/config
// (BMC_HOST / BMC_PARTNER_KEY). The key is sent as the "bmc-partner-key"
// request header, confirmed against the live gateway by the #1389 spike.
package bmc

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// operatorSlug is the BMC path segment for SNCB/NMBS. Rail only, SNCB only
// (issue #1388) — no reason to parameterise it yet.
const operatorSlug = "nmbssncb"

// ErrNotConfigured is returned when no BMC_PARTNER_KEY is set. Callers
// degrade gracefully rather than failing, matching internal/mailer.
var ErrNotConfigured = errors.New("bmc: no partner key configured")

// RateLimitedError is returned on an HTTP 429. The gateway carries no
// RateLimit-* headers — Retry-After is the only backoff signal (issue
// #1389), so it is surfaced here for the caller to honour.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("bmc: rate limited, retry after %s", e.RetryAfter)
}

// StaticOptions arms a conditional GET against the static feed. Both fields
// come from the previous import's stored validators; either or both may be
// empty on the first run.
type StaticOptions struct {
	ETag         string
	LastModified string
}

// StaticResult is a completed static-feed fetch.
type StaticResult struct {
	// Body is the raw zip. Empty when NotModified is true.
	Body []byte
	// ETag / LastModified echo the response validators, to be stored for the
	// next run's conditional GET.
	ETag         string
	LastModified string
	// NotModified is true on a 304 — the daily feed was unchanged and the
	// import is a no-op.
	NotModified bool
}

// FeedTripUpdate and FeedAlert are the BMC gateway's GTFS-Realtime feed path
// segments for SNCB/NMBS (issue #1393).
const (
	FeedTripUpdate = "rt/trip-update"
	FeedAlert      = "rt/alert"
)

// UpstreamError is a non-2xx, non-429 response from the realtime endpoint.
// Distinct from a generic error so callers can tell a 5xx (transient,
// worth backing off and retrying next poll) from anything else (a real bug
// worth surfacing to the monitoring page).
type UpstreamError struct {
	StatusCode int
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("bmc: unexpected status %d", e.StatusCode)
}

// RealtimeResult is a completed GTFS-Realtime feed fetch. Body is always
// protobuf — FetchRealtime asserts the response Content-Type rather than
// trusting the ?format=protobuf query param silently, since the gateway is
// documented (issue #1389) to serve JSON by default.
type RealtimeResult struct {
	Body []byte
}

// Client fetches feeds from the BMC gateway.
type Client interface {
	// FetchStatic downloads the SNCB GTFS static zip, honouring the
	// conditional-GET validators in opts.
	FetchStatic(ctx context.Context, opts StaticOptions) (*StaticResult, error)
	// FetchRealtime downloads one GTFS-Realtime feed (FeedTripUpdate or
	// FeedAlert) for SNCB/NMBS, always as protobuf.
	FetchRealtime(ctx context.Context, feed string) (*RealtimeResult, error)
}
