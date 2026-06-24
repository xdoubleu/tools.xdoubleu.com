package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

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
	books, err := s.books.ListBooksWithISBN13(ctx)
	if err != nil {
		return 0, err
	}

	total := len(books)
	if onProgress != nil {
		onProgress(0, total)
	}

	var errs []error
	refreshed := 0

	for i, book := range books {
		if bookErr := s.resyncBook(ctx, logger, book.ID, *book.ISBN13); bookErr != nil {
			logger.ErrorContext(ctx, "failed to resync book from open library",
				slog.String("bookID", book.ID.String()),
				slog.String("isbn13", *book.ISBN13),
				slog.Any("error", bookErr),
			)
			errs = append(errs, bookErr)
		} else {
			refreshed++
		}

		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	return refreshed, errors.Join(errs...)
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

	if dbErr := s.books.RefreshBookExternalData(
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
