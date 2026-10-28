// Package webfetch is the bounded http/https fetcher for user-supplied URLs
// (articles, feeds, PDFs, images), with size caps and conditional GETs. It
// dials through [safedial], so it never reaches non-public addresses.
package webfetch

import (
	"context"
	"errors"
	"fmt"
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

// StatusError is returned on a non-2xx response and unwraps to ErrStatus;
// errors.As reaches the exact code.
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %d", ErrStatus, e.Code)
}

func (e *StatusError) Unwrap() error {
	return ErrStatus
}

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
