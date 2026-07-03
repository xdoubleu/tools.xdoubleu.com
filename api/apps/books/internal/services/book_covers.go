package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
)

// ErrCoverNotFound is returned by GetBookCover when no cover is available for
// the book (either the book has no stored cover URL, or Open Library returned
// 404 for that URL).
var ErrCoverNotFound = errors.New("cover not found")

// coverPresignTTL is how long the presigned cover URL is valid. The browser
// and CDN can cache within this window.
const coverPresignTTL = 24 * time.Hour

// GetBookCoverResult holds the outcome of a successful GetBookCover call.
type GetBookCoverResult struct {
	URL       string
	ExpiresAt time.Time
}

// GetBookCover resolves a book cover via a cache-aside strategy backed by R2:
//
//  1. Cache hit: cover.jpg exists in R2 → return a presigned GET URL.
//  2. Negative-cache hit: cover.missing marker exists → return ErrCoverNotFound.
//  3. Miss: look up the book's stored Open Library cover URL from the DB.
//     If the URL is empty → write cover.missing, return ErrCoverNotFound.
//     Else fetch the image from Open Library with ?default=false.
//     On 404 → write cover.missing, return ErrCoverNotFound.
//     On success → store the image in R2, return a presigned GET URL.
func (s *BookService) GetBookCover(
	ctx context.Context,
	bookID uuid.UUID,
) (*GetBookCoverResult, error) {
	coverKey := bookCoverKey(bookID)
	missingKey := bookCoverMissingKey(bookID)

	// 1. Cache hit.
	exists, err := s.objectStore.Exists(ctx, coverKey)
	if err != nil {
		return nil, fmt.Errorf("check cover cache: %w", err)
	}

	if exists {
		return s.presignCover(ctx, coverKey)
	}

	// 2. Negative-cache hit.
	missing, err := s.objectStore.Exists(ctx, missingKey)
	if err != nil {
		return nil, fmt.Errorf("check cover missing marker: %w", err)
	}

	if missing {
		return nil, ErrCoverNotFound
	}

	// 3. Miss — look up the book's cover URL from the DB.
	book, err := s.books.GetBookByID(ctx, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil, ErrCoverNotFound
		}

		return nil, fmt.Errorf("look up book: %w", err)
	}

	if book.CoverURL == nil || *book.CoverURL == "" {
		_ = s.writeMissingMarker(ctx, missingKey)
		return nil, ErrCoverNotFound
	}

	// Fetch the cover from Open Library.
	data, contentType, fetchErr := s.external.FetchCover(ctx, *book.CoverURL)
	if fetchErr != nil {
		if errors.Is(fetchErr, openlibrary.ErrCoverNotFound) {
			_ = s.writeMissingMarker(ctx, missingKey)
			return nil, ErrCoverNotFound
		}

		return nil, fmt.Errorf("fetch cover from open library: %w", fetchErr)
	}

	// Store the image in R2.
	if putErr := s.objectStore.Put(
		ctx,
		coverKey,
		bytes.NewReader(data),
		int64(len(data)),
		contentType,
	); putErr != nil {
		return nil, fmt.Errorf("store cover in R2: %w", putErr)
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

func (s *BookService) writeMissingMarker(ctx context.Context, key string) error {
	return s.objectStore.Put(
		context.WithoutCancel(ctx),
		key,
		bytes.NewReader([]byte{}),
		0,
		"application/octet-stream",
	)
}
