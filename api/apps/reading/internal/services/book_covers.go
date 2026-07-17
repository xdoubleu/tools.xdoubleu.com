package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrCoverNotFound is returned by GetBookCover when no cover is cached for the
// book (either the book has no stored cover URL, or the eager fetch that
// happens when the URL is set found nothing to store).
var ErrCoverNotFound = errors.New("cover not found")

// coverPresignTTL is how long the presigned cover URL is valid. The browser
// and CDN can cache within this window.
const coverPresignTTL = 24 * time.Hour

// maxCoverBytes caps the size of a downloaded cover image — a defensive limit
// against a misbehaving or malicious source returning something huge.
const maxCoverBytes = 20 * 1024 * 1024

// GetBookCoverResult holds the outcome of a successful GetBookCover call.
type GetBookCoverResult struct {
	URL       string
	ExpiresAt time.Time
}

// GetBookCover resolves a book cover purely from R2 — covers are fetched
// eagerly into R2 whenever a book's CoverURL is set or changes (see
// cacheCoverFromURL and its call sites in books.go / book_resync.go), so the
// read path here never reaches out to an external source.
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
		return nil, ErrCoverNotFound
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

// cacheCoverFromURL downloads the image at coverURL and stores it in R2 under
// bookID's cover key, replacing whatever was cached before. Called any time a
// book gains or changes a CoverURL (add-to-library, resync apply, merge
// cover-source) so the read path (GetBookCover) never needs a live fetch.
// Errors are the caller's to log — a failed cover fetch should never block the
// write it's attached to.
func (s *BookService) cacheCoverFromURL(
	ctx context.Context,
	bookID uuid.UUID,
	coverURL string,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("build cover request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
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

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return s.objectStore.Put(
		ctx,
		bookCoverKey(bookID),
		bytes.NewReader(data),
		int64(len(data)),
		contentType,
	)
}

// clearCoverCache deletes any cached cover image and negative-cache marker for
// bookID — used when a book's CoverURL is blanked (no source supplied a cover)
// so a stale image doesn't linger in R2.
func (s *BookService) clearCoverCache(ctx context.Context, bookID uuid.UUID) error {
	if err := s.objectStore.Delete(ctx, bookCoverKey(bookID)); err != nil {
		return fmt.Errorf("delete cover: %w", err)
	}
	if err := s.objectStore.Delete(ctx, bookCoverMissingKey(bookID)); err != nil {
		return fmt.Errorf("delete cover missing marker: %w", err)
	}
	return nil
}
