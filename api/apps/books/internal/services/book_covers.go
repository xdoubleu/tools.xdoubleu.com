package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/pkg/sniff"
)

// ErrCoverNotFound is returned when no cover is cached for the book.
var ErrCoverNotFound = errors.New("cover not found")

const coverPresignTTL = 24 * time.Hour

// maxCoverBytes guards against a source returning something huge.
const maxCoverBytes = 20 * 1024 * 1024

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

// EnsureCoverCached caches bookID's cover in R2 when it isn't already there,
// returning whether it is now cached. A book with no cover URL is not an
// error, just (false, nil). Callers warm covers ahead of requests that must
// not block on a live fetch (e.g. a Kobo device's cover fetch during sync).
func (s *BookService) EnsureCoverCached(
	ctx context.Context,
	bookID uuid.UUID,
) (bool, error) {
	if exists, err := s.objectStore.Exists(ctx, bookCoverKey(bookID)); err != nil {
		return false, fmt.Errorf("check cover cache: %w", err)
	} else if exists {
		return true, nil
	}

	book, err := s.books.GetBookByID(ctx, bookID)
	if err != nil {
		return false, err
	}
	if book.CoverURL == nil || *book.CoverURL == "" {
		return false, nil
	}
	if cacheErr := s.cacheCoverFromURL(ctx, bookID, *book.CoverURL); cacheErr != nil {
		return false, cacheErr
	}
	return true, nil
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

	if s.coverFetcher == nil {
		return errors.New("no cover fetcher configured")
	}
	data, err := s.coverFetcher.Fetch(ctx, coverURL, maxCoverBytes)
	if err != nil {
		return err
	}

	// Sniffed, not the upstream header, so only raster images are stored.
	contentType := sniff.ContentType(data)
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
