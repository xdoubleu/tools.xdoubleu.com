package webfetch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/safedial"
)

const (
	// DefaultMaxBytes bounds responses when Options.MaxBytes is 0.
	DefaultMaxBytes = int64(25 << 20) // 25 MiB

	requestTimeout = 30 * time.Second
	maxRedirects   = 10
)

type client struct {
	logger    *slog.Logger
	userAgent string
	http      *http.Client
}

// New returns the production Client, identifying itself to origin servers as
// userAgent. allowPrivate permits private/loopback addresses and must be false
// in production: a user-suppliable URL must never reach the container's own
// network (see [safedial]).
func New(logger *slog.Logger, userAgent string, allowPrivate bool) Client {
	return &client{
		logger:    logger,
		userAgent: userAgent,
		http:      safedial.Client(requestTimeout, maxRedirects, allowPrivate),
	}
}

func (c *client) Get(
	ctx context.Context,
	rawURL string,
	opts Options,
) (*Result, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrScheme, rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: %s", ErrScheme, parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, parsed.String(), nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if opts.Accept != "" {
		req.Header.Set("Accept", opts.Accept)
	}
	if opts.ETag != "" {
		req.Header.Set("If-None-Match", opts.ETag)
	}
	if opts.LastModified != "" {
		req.Header.Set("If-Modified-Since", opts.LastModified)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNetwork, err)
	}
	defer resp.Body.Close()

	result := &Result{ //nolint:exhaustruct // Body/NotModified filled below
		ContentType:  normalizeContentType(resp.Header.Get("Content-Type")),
		FinalURL:     resp.Request.URL.String(),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	if resp.StatusCode == http.StatusNotModified {
		result.NotModified = true
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{Code: resp.StatusCode}
	}

	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	// Read one byte past the cap to distinguish exactly-at-cap from over.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: over %d bytes", ErrTooLarge, maxBytes)
	}

	result.Body = body
	return result, nil
}

// normalizeContentType strips parameters and lowercases the media type.
func normalizeContentType(header string) string {
	if header == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		mediaType, _, _ = strings.Cut(header, ";")
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}
