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

// ResyncAllFromOpenLibrary re-fetches Open Library metadata for every catalog
// book that has an ISBN13. It overwrites cover_url and fills description and
// page_count where Open Library returns a value. It also deletes cached R2
// cover objects so the next cover request re-downloads from the refreshed URL.
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
			bookErr := s.resyncBook(egCtx, logger, b.ID, *b.ISBN13)
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
	bookID uuid.UUID,
	isbn13 string,
) error {
	detail, err := s.external.GetByISBN(ctx, isbn13)
	if err != nil {
		if errors.Is(err, openlibrary.ErrNotFound) {
			return nil
		}
		return err
	}

	coverURL := detail.CoverURL
	if coverURL == nil {
		if fallback := openlibrary.CoverURLByISBN(&isbn13); fallback != "" {
			coverURL = &fallback
		}
	}

	if dbErr := s.resyncRepo().RefreshBookExternalData(
		ctx,
		bookID,
		coverURL,
		detail.Description,
		detail.PageCount,
	); dbErr != nil {
		return dbErr
	}

	// Clear the R2 cover cache so GetBookCover re-fetches lazily.
	if delErr := s.objectStore.Delete(ctx, bookCoverKey(bookID)); delErr != nil {
		logger.WarnContext(ctx, "failed to delete cached cover",
			slog.String("bookID", bookID.String()),
			slog.Any("error", delErr),
		)
	}
	if delErr := s.objectStore.Delete(ctx, bookCoverMissingKey(bookID)); delErr != nil {
		logger.WarnContext(ctx, "failed to delete cover missing marker",
			slog.String("bookID", bookID.String()),
			slog.Any("error", delErr),
		)
	}

	return nil
}
