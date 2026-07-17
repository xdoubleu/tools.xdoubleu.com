// Package webfetch is a bounded HTTP fetcher for external web content:
// article pages, RSS feed bodies, PDFs, and images. It enforces size caps,
// supports conditional GETs, and only ever speaks http/https — best-effort
// public fetching, no paywall circumvention.
package webfetch

import (
	"context"
	"errors"
)

var (
	// ErrTooLarge is returned when the response exceeds the size cap.
	ErrTooLarge = errors.New("webfetch: response exceeds size limit")
	// ErrStatus is returned (wrapped, with the code) on a non-2xx response.
	ErrStatus = errors.New("webfetch: non-success HTTP status")
	// ErrScheme is returned for URLs that are not http or https.
	ErrScheme = errors.New("webfetch: unsupported URL scheme")
	// ErrNetwork is returned (wrapped) when the request fails at the
	// transport level — DNS, connect, TLS, timeout.
	ErrNetwork = errors.New("webfetch: request failed")
)

// Options tunes a single Get.
type Options struct {
	// ETag / LastModified arm a conditional GET (If-None-Match /
	// If-Modified-Since); a 304 yields Result.NotModified.
	ETag         string
	LastModified string
	// MaxBytes caps the response body; 0 uses the package default.
	MaxBytes int64
	// Accept sets the Accept header when non-empty.
	Accept string
}

// Result is a completed fetch.
type Result struct {
	Body []byte
	// ContentType is the media type with parameters stripped, lowercased
	// (e.g. "text/html").
	ContentType string
	// FinalURL is the URL after following redirects.
	FinalURL string
	// ETag / LastModified echo the response validators for conditional GETs.
	ETag         string
	LastModified string
	// NotModified is true on a 304 response; Body is empty.
	NotModified bool
}

// Client fetches external URLs.
type Client interface {
	Get(ctx context.Context, rawURL string, opts Options) (*Result, error)
}
