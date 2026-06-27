package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

// booksResyncSource is the narrow subset of BooksRepository used by the resync
// path. Defined as an interface so tests can stub it without a real DB.
type booksResyncSource interface {
	ListBooksMissingMetadata(ctx context.Context) ([]models.Book, error)
	GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Book, error)
	RefreshBookExternalData(
		ctx context.Context,
		bookID uuid.UUID,
		coverURL *string,
		description *string,
		pageCount *int,
		isbn13 *string,
	) error
	SetResyncStatus(
		ctx context.Context,
		bookID uuid.UUID,
		olFound bool,
		gbFound bool,
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

// ResyncAllFromOpenLibrary backfills metadata for every catalog book that is
// missing at least one of cover_url, description, or page_count. It is
// additive-only: fields that already have a value are never overwritten. Books
// where all three fields are already populated are skipped — no network call is
// made.
//
// Enrichment strategy per book:
//   - Has ISBN13: try Open Library GetByISBN first; fall back to Google Books
//     GetByISBN for any fields still missing.
//   - No ISBN13: search Open Library then Google Books by title+author. A
//     confident match (normalised title + author-surname overlap) backfills
//     metadata and writes the discovered ISBN13 so future resyncs use the
//     faster ISBN path.
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
	books, err := s.resyncRepo().ListBooksMissingMetadata(ctx)
	if err != nil {
		return 0, err
	}
	return s.resyncBooks(ctx, logger, books, false, onProgress)
}

// ResyncBooks re-fetches metadata for the given catalog book IDs. When force
// is true every field is re-queried and existing metadata is overwritten with
// whatever the provider returns; when false only missing fields are filled
// (same behaviour as ResyncAllFromOpenLibrary). Resync status (provider
// found/not-found) is recorded regardless of the force flag.
//
// onProgress semantics are the same as ResyncAllFromOpenLibrary.
func (s *BookService) ResyncBooks(
	ctx context.Context,
	logger *slog.Logger,
	ids []uuid.UUID,
	force bool,
	onProgress func(processed, total int),
) (int, error) {
	books, err := s.resyncRepo().GetBooksByIDs(ctx, ids)
	if err != nil {
		return 0, err
	}
	return s.resyncBooks(ctx, logger, books, force, onProgress)
}

// resyncBooks is the shared bounded-concurrency loop used by both
// ResyncAllFromOpenLibrary and ResyncBooks.
func (s *BookService) resyncBooks(
	ctx context.Context,
	logger *slog.Logger,
	books []models.Book,
	force bool,
	onProgress func(processed, total int),
) (int, error) {
	total := len(books)
	if onProgress != nil {
		onProgress(0, total)
	}

	// Keep concurrency low: the client-side rate limiters are the real throttle;
	// a small goroutine cap avoids piling up goroutines blocked in limiter.Wait
	// when the library is large.
	const concurrency = 5

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
			bookErr := s.resyncBook(egCtx, logger, b, force)
			if bookErr != nil {
				isbn := "<none>"
				if b.ISBN13 != nil {
					isbn = *b.ISBN13
				}
				logger.ErrorContext(egCtx, "failed to resync book",
					slog.String("bookID", b.ID.String()),
					slog.String("isbn13", isbn),
					slog.String("title", b.Title),
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
	force bool,
) error {
	needCover := force || book.CoverURL == nil || *book.CoverURL == ""
	needDesc := force || book.Description == nil || *book.Description == ""
	needPages := force || book.PageCount == nil

	// Nothing to backfill and not forced — skip all network calls.
	if !needCover && !needDesc && !needPages {
		return nil
	}

	var olFound, gbFound bool
	var resyncErr error

	if book.ISBN13 != nil && *book.ISBN13 != "" {
		olFound, gbFound, resyncErr = s.resyncBookByISBN(
			ctx, logger, book, needCover, needDesc, needPages,
		)
	} else {
		olFound, gbFound, resyncErr = s.resyncBookByTitleAuthor(
			ctx, logger, book, needCover, needDesc, needPages,
		)
	}

	// Record provider outcomes regardless of whether metadata was written.
	if statusErr := s.resyncRepo().SetResyncStatus(
		ctx, book.ID, olFound, gbFound,
	); statusErr != nil {
		logger.WarnContext(ctx, "failed to record resync status",
			slog.String("bookID", book.ID.String()),
			slog.Any("error", statusErr),
		)
	}

	return resyncErr
}

// resyncBookByISBN enriches a book that already has an ISBN13.
// It tries Open Library first, then Google Books for any fields still missing.
// Returns (olFound, gbFound, err) where the found flags indicate whether each
// provider returned a record for this ISBN.
//
//nolint:gocognit,nestif // multi-provider per-field fallback is inherently branchy
func (s *BookService) resyncBookByISBN(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	needCover, needDesc, needPages bool,
) (bool, bool, error) {
	var olFound, gbFound bool
	var coverURL, description *string
	var pageCount *int

	// --- Open Library ---
	olDetail, olErr := s.external.GetByISBN(ctx, *book.ISBN13)
	olNotFound := errors.Is(olErr, openlibrary.ErrNotFound)
	if olErr != nil && !olNotFound {
		return false, false, olErr
	}
	olFound = olDetail != nil
	if olFound {
		// Use the explicit OL cover URL only — NOT the ISBN fallback URL yet.
		// The ISBN fallback (covers.openlibrary.org/b/isbn/…) may 404 for some
		// books; we give Google Books a chance first and only fall back to the
		// OL ISBN URL as a last resort below.
		if needCover && olDetail.CoverURL != nil {
			coverURL = olDetail.CoverURL
		}
		if needDesc {
			description = olDetail.Description
		}
		if needPages {
			pageCount = olDetail.PageCount
		}
	}

	// --- Google Books fallback for anything still missing ---
	stillNeedCover := needCover && coverURL == nil
	stillNeedDesc := needDesc && description == nil
	stillNeedPages := needPages && pageCount == nil
	if (stillNeedCover || stillNeedDesc || stillNeedPages) && s.googleBooks != nil {
		gbDetail, gbErr := s.googleBooks.GetByISBN(ctx, *book.ISBN13)
		if gbErr != nil && !errors.Is(gbErr, googlebooks.ErrNotFound) {
			// Non-fatal: log and proceed with whatever OL gave us.
			logger.WarnContext(ctx, "google books ISBN lookup failed",
				slog.String("isbn13", *book.ISBN13),
				slog.Any("error", gbErr),
			)
		}
		gbFound = gbDetail != nil
		if gbFound {
			if stillNeedCover && gbDetail.CoverURL != nil {
				coverURL = gbDetail.CoverURL
				stillNeedCover = false
			}
			if stillNeedDesc && gbDetail.Description != nil {
				description = gbDetail.Description
			}
			if stillNeedPages && gbDetail.PageCount != nil {
				pageCount = gbDetail.PageCount
			}
		}
	}

	// Last-resort cover: OL ISBN-keyed URL. Only attempted when OL actually
	// returned a record (not ErrNotFound) — the covers CDN is unlikely to have
	// anything for an ISBN that the books API doesn't know either.
	if stillNeedCover && !olNotFound {
		if fallback := openlibrary.CoverURLByISBN(book.ISBN13); fallback != "" {
			coverURL = &fallback
		}
	}

	// Nothing useful from either provider — skip the DB write.
	if coverURL == nil && description == nil && pageCount == nil {
		return olFound, gbFound, nil
	}

	return olFound, gbFound,
		s.writeResyncResult(ctx, logger, book, coverURL, description, pageCount, nil)
}

// resyncBookByTitleAuthor enriches an ISBN-less book by searching for it by
// title+author. It tries Open Library first, then Google Books. On a confident
// match it backfills metadata and writes the discovered ISBN13.
// Returns (olFound, gbFound, err) where the found flags indicate whether each
// provider returned a confident title+author match.
//
//nolint:gocognit,gocyclo,cyclop,nestif // OL+GB search chains with per-field fallback
func (s *BookService) resyncBookByTitleAuthor(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	needCover, needDesc, needPages bool,
) (bool, bool, error) {
	var olFound, gbFound bool
	if book.Title == "" || len(book.Authors) == 0 {
		// Cannot do a meaningful title/author search without both.
		return false, false, nil
	}

	query := buildSearchQuery(book.Title, book.Authors)

	// --- Open Library search ---
	olResults, olErr := s.external.Search(ctx, query)
	if olErr != nil {
		logger.WarnContext(ctx, "open library title/author search failed",
			slog.String("title", book.Title),
			slog.Any("error", olErr),
		)
	}

	var matchedISBN13 *string
	var coverURL, description *string
	var pageCount *int

	for _, r := range olResults {
		if !titleAuthorMatch(book.Title, book.Authors, r.Title, r.Authors) {
			continue
		}
		// Confident OL match found.
		olFound = true
		if needCover && r.CoverURL != nil {
			coverURL = r.CoverURL
		}
		if needDesc && r.Description != nil {
			description = r.Description
		}
		if needPages && r.PageCount != nil {
			pageCount = r.PageCount
		}
		if r.ISBN13 != nil {
			matchedISBN13 = r.ISBN13
		}
		break
	}

	// --- Google Books search (for anything still missing OR if OL had no match) ---
	stillNeed := (needCover && coverURL == nil) ||
		(needDesc && description == nil) ||
		(needPages && pageCount == nil) ||
		matchedISBN13 == nil

	if stillNeed && s.googleBooks != nil {
		gbResults, gbErr := s.googleBooks.Search(ctx, query)
		if gbErr != nil {
			logger.WarnContext(ctx, "google books title/author search failed",
				slog.String("title", book.Title),
				slog.Any("error", gbErr),
			)
		}

		for _, r := range gbResults {
			if !titleAuthorMatch(book.Title, book.Authors, r.Title, r.Authors) {
				continue
			}
			// Confident GB match found — fill any remaining gaps.
			gbFound = true
			if needCover && coverURL == nil && r.CoverURL != nil {
				coverURL = r.CoverURL
			}
			if needDesc && description == nil && r.Description != nil {
				description = r.Description
			}
			if needPages && pageCount == nil && r.PageCount != nil {
				pageCount = r.PageCount
			}
			if matchedISBN13 == nil && r.ISBN13 != nil {
				matchedISBN13 = r.ISBN13
			}
			break
		}
	}

	if coverURL == nil && description == nil && pageCount == nil &&
		matchedISBN13 == nil {
		// Neither provider found anything useful — nothing to write.
		return olFound, gbFound, nil
	}

	return olFound, gbFound, s.writeResyncResult(
		ctx, logger, book, coverURL, description, pageCount, matchedISBN13,
	)
}

// writeResyncResult persists the enriched fields and busts the cover cache
// when a new cover URL was resolved.
func (s *BookService) writeResyncResult(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	coverURL *string,
	description *string,
	pageCount *int,
	isbn13 *string,
) error {
	if dbErr := s.resyncRepo().RefreshBookExternalData(
		ctx,
		book.ID,
		coverURL,
		description,
		pageCount,
		isbn13,
	); dbErr != nil {
		return dbErr
	}

	// Bust the R2 cover cache whenever we resolved a fresh cover URL so that
	// the next request downloads the updated image. This covers both the
	// additive path (book had no cover) and the force-refresh path (book had
	// a cover but we replaced it).
	if coverURL != nil {
		s.bustCoverCache(ctx, logger, book.ID)
	}

	return nil
}

// buildSearchQuery builds a search query string for title+first-author searches.
func buildSearchQuery(title string, authors []string) string {
	// Use only the first author to keep the query focused.
	author := ""
	if len(authors) > 0 {
		author = authors[0]
	}
	if author == "" {
		return fmt.Sprintf("intitle:%q", title)
	}
	return fmt.Sprintf("intitle:%q inauthor:%q", title, author)
}

// titleAuthorMatch returns true when resultTitle normalises to the same string
// as bookTitle AND at least one of resultAuthors shares a normalised last name
// with one of bookAuthors. Returns false when either title normalises to "".
func titleAuthorMatch(
	bookTitle string,
	bookAuthors []string,
	resultTitle string,
	resultAuthors []string,
) bool {
	nt := normalizeTitle(bookTitle)
	if nt == "" {
		return false
	}
	if normalizeTitle(resultTitle) != nt {
		return false
	}

	// Build set of normalised last names from the library book.
	bookLastNames := make(map[string]struct{}, len(bookAuthors))
	for _, a := range bookAuthors {
		if n := normalizeAuthor(a); n != "" {
			bookLastNames[n] = struct{}{}
		}
	}
	if len(bookLastNames) == 0 {
		return false
	}

	// Check for overlap with the result's authors.
	for _, a := range resultAuthors {
		if n := normalizeAuthor(a); n != "" {
			if _, ok := bookLastNames[n]; ok {
				return true
			}
		}
	}
	return false
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
