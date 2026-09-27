package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/safedial"
)

// ErrCoverNotFound is returned when no cover is cached for the book.
var ErrCoverNotFound = errors.New("cover not found")

const coverPresignTTL = 24 * time.Hour

// maxCoverBytes guards against a source returning something huge.
const maxCoverBytes = 20 * 1024 * 1024

// coverFetchTimeout: GetBookCover's self-heal fetches inline in the public
// cover handler, so a dead source must fail fast rather than outlive the
// server's write timeout.
const coverFetchTimeout = 5 * time.Second

const maxCoverRedirects = 5

// newCoverClient fetches cover URLs, which can be user-supplied; private
// addresses are reachable only outside production.
func newCoverClient(env string) *http.Client {
	return safedial.Client(
		coverFetchTimeout, maxCoverRedirects, env != config.ProdEnv,
	)
}

// GetBookCoverResult holds the outcome of a successful GetBookCover call.
type GetBookCoverResult struct {
	URL       string
	ExpiresAt time.Time
}

// GetBookCover reads the cover from R2. On a miss it retries the best-effort
// eager fetch once, so a transient failure doesn't leave the book coverless
// until the next resync.
func (s *BookService) GetBookCover(
	ctx context.Context,
	bookID uuid.UUID,
) (*GetBookCoverResult, error) {
	coverKey := bookCoverKey(bookID)

	exists, err := s.objectStore.Exists(ctx, coverKey)
	if err != nil {
		return nil, fmt.Errorf("check cover cache: %w", err)
	}
	if !exists {
		book, bookErr := s.books.GetBookByID(ctx, bookID)
		if bookErr != nil || book.CoverURL == nil || *book.CoverURL == "" {
			return nil, ErrCoverNotFound
		}
		if cacheErr := s.cacheCoverFromURL(ctx, bookID, *book.CoverURL); cacheErr != nil {
			return nil, ErrCoverNotFound
		}
	}

	return s.presignCover(ctx, coverKey)
}

func (s *BookService) presignCover(
	ctx context.Context,
	key string,
) (*GetBookCoverResult, error) {
	url, err := s.objectStore.PresignGet(ctx, key, coverPresignTTL)
	if err != nil {
		return nil, fmt.Errorf("presign cover: %w", err)
	}

	return &GetBookCoverResult{
		URL:       url,
		ExpiresAt: time.Now().Add(coverPresignTTL),
	}, nil
}

// cacheCoverFromURL stores coverURL's image as bookID's cover, so the read
// path never needs a live fetch. Callers log errors; a cover failure never
// blocks the write.
func (s *BookService) cacheCoverFromURL(
	ctx context.Context,
	bookID uuid.UUID,
	coverURL string,
) error {
	parsed, err := url.Parse(coverURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("cover url %q: only http(s) is allowed", coverURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("build cover request: %w", err)
	}

	client := s.coverClient
	if client == nil {
		client = newCoverClient(config.ProdEnv)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch cover: %s returned %d", coverURL, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes+1))
	if err != nil {
		return fmt.Errorf("read cover body: %w", err)
	}
	if len(data) > maxCoverBytes {
		return fmt.Errorf("cover from %s exceeds %d bytes", coverURL, maxCoverBytes)
	}

	// Sniffed, not the upstream header, so only raster images are stored.
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("cover from %s is %s, not an image", coverURL, contentType)
	}

	return s.objectStore.Put(
		ctx,
		bookCoverKey(bookID),
		bytes.NewReader(data),
		int64(len(data)),
		contentType,
	)
}

// clearCoverCache deletes a book's cached cover and negative-cache marker.
func (s *BookService) clearCoverCache(ctx context.Context, bookID uuid.UUID) error {
	if err := s.objectStore.Delete(ctx, bookCoverKey(bookID)); err != nil {
		return fmt.Errorf("delete cover: %w", err)
	}
	if err := s.objectStore.Delete(ctx, bookCoverMissingKey(bookID)); err != nil {
		return fmt.Errorf("delete cover missing marker: %w", err)
	}
	return nil
}
