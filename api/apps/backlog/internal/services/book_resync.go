package services

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

// booksResyncSource is the narrow subset of BooksRepository used by the resync
// path. Defined as an interface so tests can stub it without a real DB.
type booksResyncSource interface {
	ListBooksWithISBN13(ctx context.Context) ([]models.Book, error)
	RefreshBookExternalData(
		ctx context.Context,
		bookID uuid.UUID,
		coverURL *string,
		description *string,
		pageCount *int,
	) error
}

// resyncRepo returns the books repo to use for resync operations.
// Tests may set BookService.booksResync to override the real repository.
func (s *BookService) resyncRepo() booksResyncSource {
	if s.booksResync != nil {
		return s.booksResync
	}
	return s.books
}

// ResyncAllFromOpenLibrary backfills Open Library metadata for every catalog
// book that has an ISBN13. It is additive-only: fields that already have a
// value (cover_url, description, page_count) are never overwritten. Books
// where all three fields are already populated are skipped entirely — no
// network call is made. When a missing cover is fetched, the R2 cover cache is
// busted so the next cover request re-downloads the fresh image; if no cover is
// found the cached cover (if any) is left untouched.
//
// onProgress is called with (processed, total) after each book attempt so that
// callers can stream progress updates to connected clients. The first call is
// always (0, total) to signal the total count before processing starts. Pass
// nil to skip progress reporting.
//
// A per-book failure is logged and collected but does not abort the batch.
// The function returns the count of books successfully refreshed and a joined
// error of any per-book failures.
func (s *BookService) ResyncAllFromOpenLibrary(
	ctx context.Context,
	logger *slog.Logger,
	onProgress func(processed, total int),
) (int, error) {
	books, err := s.resyncRepo().ListBooksWithISBN13(ctx)
	if err != nil {
		return 0, err
	}

	total := len(books)
	if onProgress != nil {
		onProgress(0, total)
	}

	const concurrency = 10

	var (
		mu        sync.Mutex
		errs      []error
		refreshed atomic.Int64
		processed atomic.Int64
	)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrency)

	for _, book := range books {
		b := book
		eg.Go(func() error {
			bookErr := s.resyncBook(egCtx, logger, b)
			if bookErr != nil {
				logger.ErrorContext(egCtx, "failed to resync book from open library",
					slog.String("bookID", b.ID.String()),
					slog.String("isbn13", *b.ISBN13),
					slog.Any("error", bookErr),
				)
				mu.Lock()
				errs = append(errs, bookErr)
				mu.Unlock()
			} else {
				refreshed.Add(1)
			}

			if onProgress != nil {
				onProgress(int(processed.Add(1)), total)
			}
			return nil
		})
	}

	_ = eg.Wait()

	return int(refreshed.Load()), errors.Join(errs...)
}

func (s *BookService) resyncBook(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
) error {
	needCover := book.CoverURL == nil || *book.CoverURL == ""
	needDesc := book.Description == nil || *book.Description == ""
	needPages := book.PageCount == nil

	// Nothing to backfill — skip the network call entirely.
	if !needCover && !needDesc && !needPages {
		return nil
	}

	detail, err := s.external.GetByISBN(ctx, *book.ISBN13)
	if err != nil {
		if errors.Is(err, openlibrary.ErrNotFound) {
			return nil
		}
		return err
	}

	// Only populate the fields that are actually missing.
	var coverURL *string
	if needCover {
		coverURL = resolveCoverURL(book.ISBN13, detail)
	}

	var description *string
	if needDesc {
		description = detail.Description
	}

	var pageCount *int
	if needPages {
		pageCount = detail.PageCount
	}

	if dbErr := s.resyncRepo().RefreshBookExternalData(
		ctx,
		book.ID,
		coverURL,
		description,
		pageCount,
	); dbErr != nil {
		return dbErr
	}

	// Bust the R2 cover cache only when we actually resolved a fresh cover URL.
	// Leaving the cache intact when no cover was found preserves any previously
	// downloaded cover image.
	if needCover && coverURL != nil {
		s.bustCoverCache(ctx, logger, book.ID)
	}

	return nil
}

// resolveCoverURL picks the best cover URL from an Open Library detail
// response, falling back to the ISBN-based URL when the detail carries none.
// Returns nil when no URL can be found.
func resolveCoverURL(
	isbn13 *string,
	detail *openlibrary.ExternalBook,
) *string {
	if detail.CoverURL != nil {
		return detail.CoverURL
	}
	if fallback := openlibrary.CoverURLByISBN(isbn13); fallback != "" {
		return &fallback
	}
	return nil
}

// bustCoverCache deletes both the cached cover image and the negative-cache
// missing marker from R2 so that GetBookCover re-fetches on the next request.
// Deletion errors are non-fatal and are logged at WARN level.
func (s *BookService) bustCoverCache(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
) {
	if delErr := s.objectStore.Delete(ctx, bookCoverKey(bookID)); delErr != nil {
		logger.WarnContext(ctx, "failed to delete cached cover",
			slog.String("bookID", bookID.String()),
			slog.Any("error", delErr),
		)
	}
	if delErr := s.objectStore.Delete(
		ctx, bookCoverMissingKey(bookID),
	); delErr != nil {
		logger.WarnContext(ctx, "failed to delete cover missing marker",
			slog.String("bookID", bookID.String()),
			slog.Any("error", delErr),
		)
	}
}
