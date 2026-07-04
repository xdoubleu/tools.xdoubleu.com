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

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
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
		title *string,
		authors []string,
	) error
	SetResyncStatus(
		ctx context.Context,
		bookID uuid.UUID,
		olFound bool,
		gbFound bool,
		ucFound bool,
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

// resyncResolution collects the metadata resolved for a single book across one
// or more provider lookups. It is pure data — no DB writes happen until
// resyncBook calls writeResyncResult.
type resyncResolution struct {
	coverURL    *string
	description *string
	pageCount   *int
	// isbn13 is set only when a new ISBN was discovered (title/author search
	// path for ISBN-less books). Never set when the book already has an ISBN.
	isbn13 *string
	// title and authors are always overwritten when a provider returns a
	// non-empty value. They are set independently of the needCover/needDesc/
	// needPages gates so the catalog stays accurate even for fully-enriched books.
	title   *string
	authors []string
	olFound bool
	gbFound bool
	ucFound bool
}

// ResyncAllFromOpenLibrary backfills metadata for every catalog book that is
// missing at least one of cover_url, description, or page_count. It is
// additive-only: fields that already have a value are never overwritten. Books
// where all three fields are already populated are skipped — no network call is
// made.
//
// Enrichment strategy per book:
//   - Has ISBN13: try Open Library GetByISBN first; fall back to Google Books
//     GetByISBN for any fields still missing. If neither ISBN lookup resolves
//     all gaps AND the book has a title+authors, a title+author search on both
//     providers is used as a final redundancy pass.
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

// resyncBook orchestrates the full enrichment pipeline for a single catalog
// book:
//
//  1. Determine which fields need filling (cover, description, page count).
//     If nothing is needed and force is false, return immediately.
//
//  2. If the book has an ISBN, call resolveByISBN to attempt a lookup against
//     Open Library then Google Books using the ISBN.
//
//  3. If gaps remain after the ISBN path (or the book has no ISBN at all), and
//     the book has a usable title+authors, call resolveByTitleAuthor as a
//     fallback. Results are merged — only empty slots are filled.
//
//  4. Record provider outcomes (olFound/gbFound) via SetResyncStatus.
//
//  5. If any metadata was resolved, write it via writeResyncResult (one DB call).
//
//nolint:cyclop,gocognit,gocyclo,nestif // two-phase resolve with merge
func (s *BookService) resyncBook(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	force bool,
) error {
	needCover := force || book.CoverURL == nil || *book.CoverURL == ""
	needDesc := force || book.Description == nil || *book.Description == ""
	needPages := force || book.PageCount == nil
	hasISBN := book.ISBN13 != nil && *book.ISBN13 != ""
	needISBN := !hasISBN

	// Nothing to backfill and not forced — skip all network calls.
	// needISBN is included: an ISBN-less book with otherwise-complete metadata
	// still deserves a title/author search to discover and persist its ISBN.
	if !needCover && !needDesc && !needPages && !needISBN {
		return nil
	}

	var res resyncResolution

	// --- Phase 1: ISBN lookup (OL then GB) ---
	if hasISBN {
		isbnRes, err := s.resolveByISBN(
			ctx,
			logger,
			book,
			needCover,
			needDesc,
			needPages,
		)
		if err != nil {
			// Hard provider error (not ErrNotFound) — propagate without writing.
			return err
		}
		res = isbnRes
	}

	// --- Phase 2: title+author / title-only fallback ---
	// Trigger when: the book has no ISBN at all, OR the ISBN phase left metadata
	// gaps AND the book has a usable title. Authors are optional — books with
	// missing or unreliable author data are handled by a title-only sub-pass
	// inside resolveByTitleAuthor.
	stillNeedCover := needCover && res.coverURL == nil
	stillNeedDesc := needDesc && res.description == nil
	stillNeedPages := needPages && res.pageCount == nil
	hasTitle := book.Title != ""

	if hasTitle &&
		(needISBN || stillNeedCover || stillNeedDesc || stillNeedPages) {
		taRes := s.resolveByTitleAuthor(
			ctx, logger, book,
			needISBN, stillNeedCover, stillNeedDesc, stillNeedPages,
		)

		// Merge: fill only empty slots.
		if res.coverURL == nil {
			res.coverURL = taRes.coverURL
		}
		if res.description == nil {
			res.description = taRes.description
		}
		if res.pageCount == nil {
			res.pageCount = taRes.pageCount
		}
		// Discovered ISBN is only relevant for books that had no ISBN13 — books
		// that already have an ISBN must not have it silently replaced.
		if !hasISBN && res.isbn13 == nil {
			res.isbn13 = taRes.isbn13
		}
		// Title/authors from the title+author phase only fill in if the ISBN
		// phase didn't already provide them (both phases prefer the first
		// provider that returns a non-empty value).
		if res.title == nil {
			res.title = taRes.title
		}
		if len(res.authors) == 0 {
			res.authors = taRes.authors
		}
		res.olFound = res.olFound || taRes.olFound
		res.gbFound = res.gbFound || taRes.gbFound
		res.ucFound = res.ucFound || taRes.ucFound
	}

	// Record provider outcomes regardless of whether metadata was written.
	if statusErr := s.resyncRepo().SetResyncStatus(
		ctx, book.ID, res.olFound, res.gbFound, res.ucFound,
	); statusErr != nil {
		logger.WarnContext(ctx, "failed to record resync status",
			slog.String("bookID", book.ID.String()),
			slog.Any("error", statusErr),
		)
	}

	// Nothing useful from either provider — skip the DB write.
	if res.coverURL == nil && res.description == nil &&
		res.pageCount == nil && res.isbn13 == nil &&
		res.title == nil && len(res.authors) == 0 {
		return nil
	}

	return s.writeResyncResult(
		ctx, logger, book,
		res.coverURL, res.description, res.pageCount, res.isbn13,
		res.title, res.authors,
	)
}

// resolveByISBN enriches a book that already has an ISBN13 by querying Open
// Library and, when fields are still missing, Google Books. It returns a
// resyncResolution without writing anything to the database.
//
//nolint:gocognit,gocyclo,cyclop,nestif,funlen // multi-provider fallback
func (s *BookService) resolveByISBN(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	needCover, needDesc, needPages bool,
) (resyncResolution, error) {
	var res resyncResolution

	// --- Open Library ---
	olDetail, olErr := s.external.GetByISBN(ctx, *book.ISBN13)
	olNotFound := errors.Is(olErr, openlibrary.ErrNotFound)
	if olErr != nil && !olNotFound {
		return resyncResolution{}, olErr
	}
	res.olFound = olDetail != nil
	if res.olFound {
		// Use the explicit OL cover URL only — NOT the ISBN fallback URL yet.
		// The ISBN fallback (covers.openlibrary.org/b/isbn/…) may 404 for some
		// books; we give Google Books a chance first and only fall back to the
		// OL ISBN URL as a last resort below.
		if needCover && olDetail.CoverURL != nil {
			res.coverURL = olDetail.CoverURL
		}
		if needDesc {
			res.description = olDetail.Description
		}
		if needPages {
			res.pageCount = olDetail.PageCount
		}
		if olDetail.Title != "" {
			res.title = &olDetail.Title
		}
		if len(olDetail.Authors) > 0 {
			res.authors = olDetail.Authors
		}
	}

	// --- Google Books fallback for anything still missing ---
	stillNeedCover := needCover && res.coverURL == nil
	stillNeedDesc := needDesc && res.description == nil
	stillNeedPages := needPages && res.pageCount == nil
	if (stillNeedCover || stillNeedDesc || stillNeedPages) && s.googleBooks != nil {
		gbDetail, gbErr := s.googleBooks.GetByISBN(ctx, *book.ISBN13)
		if gbErr != nil && !errors.Is(gbErr, googlebooks.ErrNotFound) {
			// Non-fatal: log and proceed with whatever OL gave us.
			logger.WarnContext(ctx, "google books ISBN lookup failed",
				slog.String("isbn13", *book.ISBN13),
				slog.Any("error", gbErr),
			)
		}
		res.gbFound = gbDetail != nil
		if res.gbFound {
			if stillNeedCover && gbDetail.CoverURL != nil {
				res.coverURL = gbDetail.CoverURL
			}
			if stillNeedDesc && gbDetail.Description != nil {
				res.description = gbDetail.Description
			}
			if stillNeedPages && gbDetail.PageCount != nil {
				res.pageCount = gbDetail.PageCount
			}
			// Fill title/authors from GB if OL didn't provide them.
			if res.title == nil && gbDetail.Title != "" {
				res.title = &gbDetail.Title
			}
			if len(res.authors) == 0 && len(gbDetail.Authors) > 0 {
				res.authors = gbDetail.Authors
			}
		}
	}

	// --- UniCat fallback for anything still missing (Dutch/Flemish books) ---
	stillNeedCover2 := needCover && res.coverURL == nil
	stillNeedDesc2 := needDesc && res.description == nil
	stillNeedPages2 := needPages && res.pageCount == nil
	if (stillNeedDesc2 || stillNeedPages2 || res.title == nil) &&
		s.uniCat != nil {
		ucDetail, ucErr := s.uniCat.GetByISBN(ctx, *book.ISBN13)
		if ucErr != nil && !errors.Is(ucErr, unicat.ErrNotFound) {
			logger.WarnContext(ctx, "unicat ISBN lookup failed",
				slog.String("isbn13", *book.ISBN13),
				slog.Any("error", ucErr),
			)
		}
		if ucDetail != nil {
			res.ucFound = true
			if stillNeedDesc2 && ucDetail.Description != nil {
				res.description = ucDetail.Description
			}
			if stillNeedPages2 && ucDetail.PageCount != nil {
				res.pageCount = ucDetail.PageCount
			}
			if res.title == nil && ucDetail.Title != "" {
				res.title = &ucDetail.Title
			}
			if len(res.authors) == 0 && len(ucDetail.Authors) > 0 {
				res.authors = ucDetail.Authors
			}
		}
	}
	_ = stillNeedCover2 // UniCat has no cover images; kept for symmetry

	// Last-resort cover: OL ISBN-keyed URL. Only attempted when OL actually
	// returned a record (not ErrNotFound) — the covers CDN is unlikely to have
	// anything for an ISBN that the books API doesn't know either.
	if needCover && res.coverURL == nil && !olNotFound {
		if fallback := openlibrary.CoverURLByISBN(book.ISBN13); fallback != "" {
			res.coverURL = &fallback
		}
	}

	return res, nil
}

// resolveByTitleAuthor enriches a book using free-text search. It runs two
// sub-passes in order:
//
//  1. Title+author pass (only when book.Authors is non-empty): queries each
//     provider with intitle+inauthor and accepts results via titleAuthorMatch.
//     This is the highest-confidence path.
//
//  2. Title-only fallback: triggered when the first pass left gaps or was
//     skipped (missing/unreliable authors). Uses selectTitleOnlyMatch with an
//     ambiguity guard so that titles shared by genuinely different books are
//     never silently mismatched.
//
// Provider errors are logged and suppressed — the function always returns
// whatever it could gather; the caller decides whether to proceed with partial
// data.
//
//nolint:gocognit,gocyclo,cyclop,nestif,funlen // two-pass search with merge
func (s *BookService) resolveByTitleAuthor(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	needISBN, needCover, needDesc, needPages bool,
) resyncResolution {
	var res resyncResolution

	if book.Title == "" {
		// Cannot search without a title.
		return res
	}

	// --- Pass 1: title+author search (highest confidence) ---
	if len(book.Authors) > 0 {
		query := buildSearchQuery(book.Title, book.Authors)

		// Open Library
		olResults, olErr := s.external.Search(ctx, query)
		if olErr != nil {
			logger.WarnContext(ctx, "open library title/author search failed",
				slog.String("title", book.Title),
				slog.Any("error", olErr),
			)
		}

		for _, r := range olResults {
			if !titleAuthorMatch(book.Title, book.Authors, r.Title, r.Authors) {
				continue
			}
			res.olFound = true
			if needCover && r.CoverURL != nil {
				res.coverURL = r.CoverURL
			}
			if needDesc && r.Description != nil {
				res.description = r.Description
			}
			if needPages && r.PageCount != nil {
				res.pageCount = r.PageCount
			}
			if r.ISBN13 != nil {
				normalized := normalizeISBN(*r.ISBN13)
				res.isbn13 = &normalized
			}
			if r.Title != "" {
				res.title = &r.Title
			}
			if len(r.Authors) > 0 {
				res.authors = r.Authors
			}
			break
		}

		// Google Books (anything still missing OR OL had no match)
		stillNeedGB := (needCover && res.coverURL == nil) ||
			(needDesc && res.description == nil) ||
			(needPages && res.pageCount == nil) ||
			res.isbn13 == nil

		if stillNeedGB && s.googleBooks != nil {
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
				res.gbFound = true
				if needCover && res.coverURL == nil && r.CoverURL != nil {
					res.coverURL = r.CoverURL
				}
				if needDesc && res.description == nil && r.Description != nil {
					res.description = r.Description
				}
				if needPages && res.pageCount == nil && r.PageCount != nil {
					res.pageCount = r.PageCount
				}
				if res.isbn13 == nil && r.ISBN13 != nil {
					normalized := normalizeISBN(*r.ISBN13)
					res.isbn13 = &normalized
				}
				if res.title == nil && r.Title != "" {
					res.title = &r.Title
				}
				if len(res.authors) == 0 && len(r.Authors) > 0 {
					res.authors = r.Authors
				}
				break
			}
		}

		// UniCat (Dutch/Flemish books)
		stillNeedUC := (needDesc && res.description == nil) ||
			(needPages && res.pageCount == nil) ||
			res.isbn13 == nil

		if stillNeedUC && s.uniCat != nil {
			ucResults, ucErr := s.uniCat.Search(ctx, query)
			if ucErr != nil {
				logger.WarnContext(ctx, "unicat title/author search failed",
					slog.String("title", book.Title),
					slog.Any("error", ucErr),
				)
			}

			for _, r := range ucResults {
				if !titleAuthorMatch(book.Title, book.Authors, r.Title, r.Authors) {
					continue
				}
				res.ucFound = true
				if needDesc && res.description == nil && r.Description != nil {
					res.description = r.Description
				}
				if needPages && res.pageCount == nil && r.PageCount != nil {
					res.pageCount = r.PageCount
				}
				if res.isbn13 == nil && r.ISBN13 != nil {
					normalized := normalizeISBN(*r.ISBN13)
					res.isbn13 = &normalized
				}
				if res.title == nil && r.Title != "" {
					res.title = &r.Title
				}
				if len(res.authors) == 0 && len(r.Authors) > 0 {
					res.authors = r.Authors
				}
				break
			}
		}
	}

	// --- Pass 2: title-only fallback ---
	// Runs when Pass 1 was skipped (no authors) OR left gaps that still matter.
	stillNeedTO := (needISBN && res.isbn13 == nil) ||
		(needCover && res.coverURL == nil) ||
		(needDesc && res.description == nil) ||
		(needPages && res.pageCount == nil)

	if stillNeedTO {
		toRes := s.resolveByTitleOnly(ctx, logger, book, needCover, needDesc, needPages)
		if res.coverURL == nil {
			res.coverURL = toRes.coverURL
		}
		if res.description == nil {
			res.description = toRes.description
		}
		if res.pageCount == nil {
			res.pageCount = toRes.pageCount
		}
		if res.isbn13 == nil {
			res.isbn13 = toRes.isbn13
		}
		if res.title == nil {
			res.title = toRes.title
		}
		if len(res.authors) == 0 {
			res.authors = toRes.authors
		}
		res.olFound = res.olFound || toRes.olFound
		res.gbFound = res.gbFound || toRes.gbFound
		res.ucFound = res.ucFound || toRes.ucFound
	}

	return res
}

// resolveByTitleOnly queries each provider with a title-only search
// (intitle:"...") and returns the first result accepted by selectTitleOnlyMatch.
// It is the inner engine for Pass 2 of resolveByTitleAuthor.
//
// Provider errors are logged and suppressed.
//
//nolint:gocognit,gocyclo,cyclop,nestif,funlen // 3-provider title-only fallback
func (s *BookService) resolveByTitleOnly(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	needCover, needDesc, needPages bool,
) resyncResolution {
	var res resyncResolution

	query := buildSearchQuery(book.Title, nil) // intitle:"..." only

	// --- Open Library title-only search ---
	olResults, olErr := s.external.Search(ctx, query)
	if olErr != nil {
		logger.WarnContext(ctx, "open library title-only search failed",
			slog.String("title", book.Title),
			slog.Any("error", olErr),
		)
	}

	if len(olResults) > 0 {
		cands := make([]titleOnlyCandidate, len(olResults))
		for i, r := range olResults {
			cands[i] = titleOnlyCandidate{
				title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
				coverURL: r.CoverURL, description: r.Description,
				pageCount: r.PageCount,
			}
		}
		if m, ok := selectTitleOnlyMatch(book.Title, cands); ok {
			res.olFound = true
			if needCover && m.coverURL != nil {
				res.coverURL = m.coverURL
			}
			if needDesc && m.description != nil {
				res.description = m.description
			}
			if needPages && m.pageCount != nil {
				res.pageCount = m.pageCount
			}
			if m.isbn13 != nil {
				normalized := normalizeISBN(*m.isbn13)
				res.isbn13 = &normalized
			}
			if m.title != "" {
				res.title = &m.title
			}
			if len(m.authors) > 0 {
				res.authors = m.authors
			}
		}
	}

	// --- Google Books title-only search ---
	stillNeedGB := (needCover && res.coverURL == nil) ||
		(needDesc && res.description == nil) ||
		(needPages && res.pageCount == nil) ||
		res.isbn13 == nil

	if stillNeedGB && s.googleBooks != nil {
		gbResults, gbErr := s.googleBooks.Search(ctx, query)
		if gbErr != nil {
			logger.WarnContext(ctx, "google books title-only search failed",
				slog.String("title", book.Title),
				slog.Any("error", gbErr),
			)
		}

		if len(gbResults) > 0 {
			cands := make([]titleOnlyCandidate, len(gbResults))
			for i, r := range gbResults {
				cands[i] = titleOnlyCandidate{
					title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
					coverURL: r.CoverURL, description: r.Description,
					pageCount: r.PageCount,
				}
			}
			if m, ok := selectTitleOnlyMatch(book.Title, cands); ok {
				res.gbFound = true
				if needCover && res.coverURL == nil && m.coverURL != nil {
					res.coverURL = m.coverURL
				}
				if needDesc && res.description == nil && m.description != nil {
					res.description = m.description
				}
				if needPages && res.pageCount == nil && m.pageCount != nil {
					res.pageCount = m.pageCount
				}
				if res.isbn13 == nil && m.isbn13 != nil {
					normalized := normalizeISBN(*m.isbn13)
					res.isbn13 = &normalized
				}
				if res.title == nil && m.title != "" {
					res.title = &m.title
				}
				if len(res.authors) == 0 && len(m.authors) > 0 {
					res.authors = m.authors
				}
			}
		}
	}

	// --- UniCat title-only search (Dutch/Flemish books; no cover images) ---
	stillNeedUC := (needDesc && res.description == nil) ||
		(needPages && res.pageCount == nil) ||
		res.isbn13 == nil

	if stillNeedUC && s.uniCat != nil {
		ucResults, ucErr := s.uniCat.Search(ctx, query)
		if ucErr != nil {
			logger.WarnContext(ctx, "unicat title-only search failed",
				slog.String("title", book.Title),
				slog.Any("error", ucErr),
			)
		}

		if len(ucResults) > 0 {
			cands := make([]titleOnlyCandidate, len(ucResults))
			for i, r := range ucResults {
				cands[i] = titleOnlyCandidate{ //nolint:exhaustruct // no cover in UniCat
					title:       r.Title,
					authors:     r.Authors,
					isbn13:      r.ISBN13,
					description: r.Description,
					pageCount:   r.PageCount,
				}
			}
			if m, ok := selectTitleOnlyMatch(book.Title, cands); ok {
				res.ucFound = true
				if needDesc && res.description == nil && m.description != nil {
					res.description = m.description
				}
				if needPages && res.pageCount == nil && m.pageCount != nil {
					res.pageCount = m.pageCount
				}
				if res.isbn13 == nil && m.isbn13 != nil {
					normalized := normalizeISBN(*m.isbn13)
					res.isbn13 = &normalized
				}
				if res.title == nil && m.title != "" {
					res.title = &m.title
				}
				if len(res.authors) == 0 && len(m.authors) > 0 {
					res.authors = m.authors
				}
			}
		}
	}

	return res
}

// titleOnlyCandidate holds the metadata fields from a provider search result
// used by selectTitleOnlyMatch. All three external providers expose the same
// set of fields; this common type avoids duplicating the helper per provider.
type titleOnlyCandidate struct {
	title       string
	authors     []string
	isbn13      *string
	coverURL    *string
	description *string
	pageCount   *int
}

// selectTitleOnlyMatch filters candidates to those whose normalised title
// equals bookTitle, then applies an ambiguity guard: if two or more
// title-matching candidates have non-empty, fully-disjoint normalised author
// sets (indicating genuinely different books that share a title), the function
// returns (zero, false) so that no metadata is written. When exactly one title
// match exists, or all title-matching candidates share at least one common
// author (same book in different editions), the first match is returned as
// (match, true).
//
//nolint:gocognit // pairwise disjoint-author check; split would not reduce complexity
func selectTitleOnlyMatch(
	bookTitle string,
	candidates []titleOnlyCandidate,
) (titleOnlyCandidate, bool) {
	normBook := normalizeTitle(bookTitle)
	if normBook == "" {
		return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value intended
	}

	var matching []titleOnlyCandidate
	for _, c := range candidates {
		if normalizeTitle(c.title) == normBook {
			matching = append(matching, c)
		}
	}

	switch len(matching) {
	case 0:
		return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value intended
	case 1:
		return matching[0], true
	}

	// Ambiguity guard: build per-candidate normalised author sets and check
	// for any fully-disjoint pair — a pair with no common author name is
	// strong evidence that the same title belongs to two different books.
	authorSets := make([]map[string]struct{}, len(matching))
	for i, m := range matching {
		s := make(map[string]struct{}, len(m.authors))
		for _, a := range m.authors {
			if n := normalizeAuthor(a); n != "" {
				s[n] = struct{}{}
			}
		}
		authorSets[i] = s
	}

	for i := 0; i < len(authorSets); i++ {
		if len(authorSets[i]) == 0 {
			continue // no authors → cannot determine conflict
		}
		for j := i + 1; j < len(authorSets); j++ {
			if len(authorSets[j]) == 0 {
				continue
			}
			overlap := false
			for a := range authorSets[i] {
				if _, ok := authorSets[j][a]; ok {
					overlap = true
					break
				}
			}
			if !overlap {
				// Disjoint authors for the same title — ambiguous.
				return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value
			}
		}
	}

	return matching[0], true
}

// writeResyncResult persists the enriched fields and busts the cover cache
// when a new cover URL was resolved. title and authors are written when
// non-empty — pass nil/nil to leave them unchanged.
func (s *BookService) writeResyncResult(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	coverURL *string,
	description *string,
	pageCount *int,
	isbn13 *string,
	title *string,
	authors []string,
) error {
	if dbErr := s.resyncRepo().RefreshBookExternalData(
		ctx,
		book.ID,
		coverURL,
		description,
		pageCount,
		isbn13,
		title,
		authors,
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
