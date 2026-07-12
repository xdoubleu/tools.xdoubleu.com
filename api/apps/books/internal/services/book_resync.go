package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

// ErrProposalNotFound is returned by ApplyResyncChoice when the book has no
// pending proposal — it was already applied/dismissed, or a scan never
// flagged it in the first place.
var ErrProposalNotFound = errors.New("resync proposal not found")

// booksResyncSource is the narrow subset of BooksRepository used by the resync
// path. Defined as an interface so tests can stub it without a real DB.
type booksResyncSource interface {
	ListCatalogBooks(ctx context.Context) ([]models.Book, error)
	GetBookByID(ctx context.Context, bookID uuid.UUID) (*models.Book, error)
	RefreshBookExternalData(
		ctx context.Context,
		bookID uuid.UUID,
		coverURL *string,
		description *string,
		pageCount *int,
		isbn13 *string,
		title *string,
		authors []string,
		metadataSource string,
	) error
	UpdateResyncScanStatus(
		ctx context.Context,
		bookID uuid.UUID,
		openLibraryFound *bool,
		googleBooksFound *bool,
		uniCatFound *bool,
	) error
	GetSourceStats(ctx context.Context) (*repositories.SourceStats, error)
	ListBooksInExactSources(
		ctx context.Context,
		sources []string,
	) ([]models.Book, error)
	ReplaceResyncProposals(ctx context.Context, entries map[uuid.UUID][]byte) error
	ListResyncProposals(ctx context.Context) ([]repositories.ResyncProposalRow, error)
	GetResyncProposal(
		ctx context.Context,
		bookID uuid.UUID,
	) (*repositories.ResyncProposalRow, error)
	DeleteResyncProposal(ctx context.Context, bookID uuid.UUID) error
}

// resyncRepo returns the books repo to use for resync operations.
// Tests may set BookService.booksResync to override the real repository.
func (s *BookService) resyncRepo() booksResyncSource {
	if s.booksResync != nil {
		return s.booksResync
	}
	return s.books
}

// SourceProposal is one candidate metadata set for a catalog book: either the
// current library values (Source == "") or one external provider's proposal
// ("openlibrary" | "googlebooks" | "unicat"). Zero-value fields mean the
// source didn't supply that field.
type SourceProposal struct {
	Source      string   `json:"source"`
	CoverURL    string   `json:"cover_url,omitempty"`
	Description string   `json:"description,omitempty"`
	PageCount   int      `json:"page_count,omitempty"`
	ISBN13      string   `json:"isbn13,omitempty"`
	Title       string   `json:"title,omitempty"`
	Authors     []string `json:"authors,omitempty"`
	// Differs lists which fields differ from the library values. Computed at
	// read time (never persisted), empty for the library's own SourceProposal.
	Differs []string `json:"-"`
}

// ResyncProposal pairs a catalog book with the source proposals that differ
// from it, for the admin resync wizard to step through.
type ResyncProposal struct {
	BookID  string
	Library SourceProposal
	Sources []SourceProposal
}

// BuildResyncProposals scans the whole catalog and, for every book, fetches
// each external source independently — no priority merge, every source that
// returns a match is kept as its own candidate. Two situations get a book
// flagged for the admin resync wizard to review: at least one source
// disagrees with the library, or every configured source came up empty for a
// searchable book (a coverage gap — the wizard shows these with no source
// cards so an admin can spot books that may need a new source added).
// Nothing is written to a book here. Re-running replaces the whole table, so
// books that now agree with every source (or were fixed by a prior wizard
// pass) drop out automatically.
//
// onProgress is called with (processed, total, gbQuotaReached) after each
// book, first call always (0, total, false). gbQuotaReached reports whether
// the Google Books 429 breaker has tripped for this run (see gbExceeded
// below), so callers can surface a quota-reached notice live. Pass nil to
// skip progress reporting. A per-book fetch failure is logged and collected
// but does not abort the scan.
//
// force bypasses the skip-if-known cache (see scanOptions) so every source is
// queried fresh for every book, even ones already resolved true or false —
// the escape hatch for books stuck unresolved after a rate-limit trip or a
// stale cached miss. The Google Books 429 circuit breaker still applies
// within a forced run.
func (s *BookService) BuildResyncProposals(
	ctx context.Context,
	logger *slog.Logger,
	onProgress func(processed, total int, gbQuotaReached bool),
	force bool,
) (int, error) {
	books, err := s.resyncRepo().ListCatalogBooks(ctx)
	if err != nil {
		return 0, err
	}

	total := len(books)
	if onProgress != nil {
		onProgress(0, total, false)
	}

	// Same concurrency cap as the old resync loop: the client-side rate
	// limiters are the real throttle, this just bounds in-flight goroutines.
	const concurrency = 5

	//nolint:exhaustruct // errs/mu zero values fine
	acc := &resyncAccumulator{entries: make(map[uuid.UUID][]byte)}
	var processed atomic.Int64

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrency)

	// gbExceeded trips on the first Google Books 429 and is shared across the
	// whole run: once the daily quota is gone, every further GB call in this
	// run would just error and spam logs — skip them and retry next run.
	gbExceeded := &atomic.Bool{}

	for _, book := range books {
		b := book
		eg.Go(func() error {
			// Cancelled (StartResync's Cancel RPC, or app shutdown): stop
			// picking up new books, let already in-flight ones finish.
			if egCtx.Err() != nil {
				return nil //nolint:nilerr // cancellation is not a failure
			}
			opts := &scanOptions{
				known:      knownFor(b, force),
				gbExceeded: gbExceeded,
			}
			s.scanBookForResync(egCtx, logger, b, opts, acc)
			if onProgress != nil {
				onProgress(int(processed.Add(1)), total, gbExceeded.Load())
			}
			return nil
		})
	}
	_ = eg.Wait()

	// Cancelled mid-run: books already processed keep the scan-status
	// writes recordScanStatus already committed, but the proposals table is
	// left untouched — replacing it with a partial scan's results would
	// erase proposals from books this run never got to. Not an error: the
	// caller asked to stop.
	if ctx.Err() != nil {
		return len(acc.entries), nil //nolint:nilerr // cancellation is not a failure
	}

	if err = s.resyncRepo().ReplaceResyncProposals(ctx, acc.entries); err != nil {
		return 0, err
	}

	return len(acc.entries), errors.Join(acc.errs...)
}

// resyncAccumulator collects one BuildResyncProposals run's per-book results
// under a single mutex, since scanBookForResync runs concurrently across
// books.
type resyncAccumulator struct {
	mu      sync.Mutex
	entries map[uuid.UUID][]byte
	errs    []error
}

func (a *resyncAccumulator) addEntry(bookID uuid.UUID, raw []byte) {
	a.mu.Lock()
	a.entries[bookID] = raw
	a.mu.Unlock()
}

func (a *resyncAccumulator) addError(err error) {
	a.mu.Lock()
	a.errs = append(a.errs, err)
	a.mu.Unlock()
}

// scanBookForResync fetches one book's candidate proposals from every
// configured source, records them into acc when the book should be flagged,
// and persists the scan status — the per-book unit of work BuildResyncProposals
// runs concurrently.
func (s *BookService) scanBookForResync(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
	acc *resyncAccumulator,
) {
	proposals, unresolved := s.fetchSourceProposals(ctx, logger, book, opts)
	if raw, ok := encodeIfFlagged(book, proposals); ok {
		if raw != nil {
			acc.addEntry(book.ID, raw)
		} else {
			acc.addError(fmt.Errorf("book %s: encode proposals", book.ID))
		}
	}
	statusErr := s.recordScanStatus(ctx, book, proposals, unresolved)
	if statusErr != nil {
		acc.addError(fmt.Errorf("book %s: record scan status: %w", book.ID, statusErr))
	}
}

// recordScanStatus persists one scan pass's per-source found flags on the
// book. A nil flag leaves the column unchanged in the DB (see
// UpdateResyncScanStatus) and covers every case where the source wasn't
// actually resolved this pass: not configured, the book wasn't searchable (no
// ISBN and no title), skipped because already known, or its call errored —
// unresolved carries those last two (see scanOptions / fetchByISBN).
func (s *BookService) recordScanStatus(
	ctx context.Context,
	book models.Book,
	proposals []SourceProposal,
	unresolved map[string]bool,
) error {
	attempted := (book.ISBN13 != nil && *book.ISBN13 != "") || book.Title != ""

	found := func(source string) *bool {
		if !attempted || unresolved[source] {
			return nil
		}
		f := false
		for _, p := range proposals {
			if p.Source == source {
				f = true
				break
			}
		}
		return &f
	}

	olFound := found("openlibrary")
	var gbFound, ucFound *bool
	if s.googleBooks != nil {
		gbFound = found("googlebooks")
	}
	if s.uniCat != nil {
		ucFound = found("unicat")
	}

	return s.resyncRepo().UpdateResyncScanStatus(
		ctx, book.ID, olFound, gbFound, ucFound,
	)
}

// encodeIfFlagged returns the JSON-marshaled proposals and true when the book
// should be surfaced to the wizard — either a source disagrees with the
// library, or every configured, queryable source came up empty (a coverage
// gap worth knowing about, e.g. to decide whether a new source is needed).
// Books nobody could search (no ISBN and no title) are never flagged: nothing
// was actually attempted. A nil, true result signals a marshal failure that
// the caller should log.
func encodeIfFlagged(book models.Book, proposals []SourceProposal) ([]byte, bool) {
	attempted := (book.ISBN13 != nil && *book.ISBN13 != "") || book.Title != ""
	// anyKnownFound guards against a false "not found anywhere": an
	// incremental scan (scanOptions.known) skips re-querying a source that's
	// already confirmed found, so this pass's proposals can be empty for a
	// book that's actually well covered — that must never read as a gap.
	notFoundAnywhere := attempted && len(proposals) == 0 && !anyKnownFound(book)
	if !notFoundAnywhere && !anyDiffers(book, proposals) {
		return nil, false
	}
	raw, err := json.Marshal(proposals)
	if err != nil {
		return nil, true
	}
	return raw, true
}

// anyKnownFound reports whether any source was already confirmed to have
// this book as of the last scan.
func anyKnownFound(book models.Book) bool {
	isTrue := func(b *bool) bool { return b != nil && *b }
	return isTrue(book.OpenLibraryFound) ||
		isTrue(book.GoogleBooksFound) ||
		isTrue(book.UniCatFound)
}

func anyDiffers(book models.Book, proposals []SourceProposal) bool {
	for _, p := range proposals {
		if len(computeDifferences(book, p)) > 0 {
			return true
		}
	}
	return false
}

// scanOptions gates the bulk BuildResyncProposals pass. known lets a source's
// call be skipped once that source has already been resolved for the book —
// true or false, doesn't matter, either answer is on record — with
// UpdateResyncScanStatus's preserve-on-unknown write, the found columns are a
// durable cache, so a steady-state scan only re-queries sources with no
// answer yet. BuildResyncProposals' force param leaves known empty for every
// book, bypassing the cache entirely for one run — the escape hatch for
// books stuck unresolved after a rate-limit trip or a stale cached miss
// (skip-if-known never re-checks a resolved source for drift otherwise, e.g.
// a book gaining a cover later).
// gbExceeded is Google-Books-only: it's the one source with a real daily
// quota (free tier: 1000/day), so it trips a circuit breaker for the rest of
// one BuildResyncProposals run after a 429, and stays tripped even within a
// forced run — otherwise a full-catalog force exhausts the quota and every
// subsequent GB lookup that day errors. OpenLibrary and UniCat have no such
// quota or breaker; only skip-if-known gates them.
// nil means on-demand mode (GetBookSources / ApplyBookSource): always query
// every source fresh, no skip, no breaker.
type scanOptions struct {
	known      map[string]bool
	gbExceeded *atomic.Bool
}

func gbBreakerTripped(opts *scanOptions) bool {
	return opts != nil && opts.gbExceeded != nil && opts.gbExceeded.Load()
}

func (opts *scanOptions) skipKnown(source string) bool {
	return opts != nil && opts.known[source]
}

// knownFor returns the set of sources already resolved for this book as of
// the last scan — true or false, doesn't matter, a non-nil found column means
// that source has an answer on record — or an empty set when force bypasses
// the cache for this run.
func knownFor(book models.Book, force bool) map[string]bool {
	known := map[string]bool{}
	if force {
		return known
	}
	if book.OpenLibraryFound != nil {
		known["openlibrary"] = true
	}
	if book.GoogleBooksFound != nil {
		known["googlebooks"] = true
	}
	if book.UniCatFound != nil {
		known["unicat"] = true
	}
	return known
}

// fetchSourceProposals fetches each configured provider's view of one catalog
// book, independently — no gap-filling across providers. ISBN lookups are
// used when the book has an ISBN13 (definitive match); otherwise a
// title/author search is used, gated by the same match guards the old resync
// path used (titleAuthorMatch / selectTitleOnlyMatch) so an unrelated book
// sharing a title is never proposed. The second return value names every
// source that wasn't actually resolved this pass (skipped or errored) — see
// recordScanStatus.
func (s *BookService) fetchSourceProposals(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	if book.ISBN13 != nil && *book.ISBN13 != "" {
		return s.fetchByISBN(ctx, logger, *book.ISBN13, opts)
	}
	if book.Title == "" {
		return nil, nil
	}
	return s.fetchBySearch(ctx, logger, book, opts)
}

// fetchByISBN queries every configured provider's GetByISBN independently and
// keeps every result — no fallback chaining, each provider stands on its own.
func (s *BookService) fetchByISBN(
	ctx context.Context,
	logger *slog.Logger,
	isbn13 string,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	var out []SourceProposal
	unresolved := map[string]bool{}

	if opts.skipKnown("openlibrary") {
		unresolved["openlibrary"] = true
	} else if olDetail, olErr := s.external.GetByISBN(ctx, isbn13); olErr != nil &&
		!errors.Is(olErr, openlibrary.ErrNotFound) {
		unresolved["openlibrary"] = true
		logger.WarnContext(ctx, "open library ISBN lookup failed",
			slog.String("isbn13", isbn13), slog.Any("error", olErr))
	} else if olDetail != nil {
		p := newSourceProposalFromCandidate("openlibrary", titleOnlyCandidate{
			title: olDetail.Title, authors: olDetail.Authors, isbn13: olDetail.ISBN13,
			coverURL: olDetail.CoverURL, description: olDetail.Description,
			pageCount: olDetail.PageCount,
		})
		// Last-resort ISBN-keyed cover URL when OL found the book but its
		// record has no explicit cover.
		if p.CoverURL == "" {
			p.CoverURL = openlibrary.CoverURLByISBN(&isbn13)
		}
		out = append(out, p)
	}

	if s.googleBooks != nil {
		if p, unres := s.fetchGoogleBooksByISBN(ctx, logger, isbn13, opts); unres {
			unresolved["googlebooks"] = true
		} else if p != nil {
			out = append(out, *p)
		}
	}

	if s.uniCat != nil && opts.skipKnown("unicat") {
		unresolved["unicat"] = true
	} else if s.uniCat != nil {
		ucDetail, ucErr := s.uniCat.GetByISBN(ctx, isbn13)
		if ucErr != nil && !errors.Is(ucErr, unicat.ErrNotFound) {
			unresolved["unicat"] = true
			logger.WarnContext(ctx, "unicat ISBN lookup failed",
				slog.String("isbn13", isbn13), slog.Any("error", ucErr))
		}
		if ucDetail != nil {
			out = append(
				out,
				newSourceProposalFromCandidate(
					"unicat",
					titleOnlyCandidate{ //nolint:exhaustruct // UniCat has no cover images
						title:       ucDetail.Title,
						authors:     ucDetail.Authors,
						isbn13:      ucDetail.ISBN13,
						description: ucDetail.Description,
						pageCount:   ucDetail.PageCount,
					},
				),
			)
		}
	}

	return out, unresolved
}

// fetchGoogleBooksByISBN queries Google Books for one ISBN, honoring opts'
// skip-if-known and circuit-breaker rules. Returns the proposal (nil if GB
// has no match) and whether the source is unresolved this pass (skipped,
// breaker-tripped, or errored) — see recordScanStatus.
func (s *BookService) fetchGoogleBooksByISBN(
	ctx context.Context,
	logger *slog.Logger,
	isbn13 string,
	opts *scanOptions,
) (*SourceProposal, bool) {
	if gbBreakerTripped(opts) || opts.skipKnown("googlebooks") {
		return nil, true
	}

	gbDetail, gbErr := s.googleBooks.GetByISBN(ctx, isbn13)
	if gbErr != nil && !errors.Is(gbErr, googlebooks.ErrNotFound) {
		if opts != nil && opts.gbExceeded != nil &&
			errors.Is(gbErr, googlebooks.ErrRateLimited) {
			opts.gbExceeded.Store(true)
		}
		logger.WarnContext(ctx, "google books ISBN lookup failed",
			slog.String("isbn13", isbn13), slog.Any("error", gbErr))
		return nil, true
	}
	if gbDetail == nil {
		return nil, false
	}

	p := newSourceProposalFromCandidate("googlebooks", titleOnlyCandidate{
		title: gbDetail.Title, authors: gbDetail.Authors, isbn13: gbDetail.ISBN13,
		coverURL: gbDetail.CoverURL, description: gbDetail.Description,
		pageCount: gbDetail.PageCount,
	})
	return &p, false
}

// fetchBySearch queries every configured provider's Search independently and
// keeps the first accepted match per provider: title+author matching
// (titleAuthorMatch) when the book has authors, otherwise the ambiguity-
// guarded title-only match (selectTitleOnlyMatch) — the same guards the old
// resync path used.
func (s *BookService) fetchBySearch(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	return s.searchProviders(
		ctx, logger, book.Title, buildSearchQuery(book.Title, book.Authors),
		func(candidates []titleOnlyCandidate) (titleOnlyCandidate, bool) {
			return matchSearchResult(book, candidates)
		},
		opts,
	)
}

// searchProviders queries every configured provider's Search with one query
// and keeps at most one candidate per provider, selected by pick.
//
//nolint:gocognit // The function is long but the logic is straightforward
func (s *BookService) searchProviders(
	ctx context.Context,
	logger *slog.Logger,
	logTitle string,
	query string,
	pick func([]titleOnlyCandidate) (titleOnlyCandidate, bool),
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	var out []SourceProposal
	unresolved := map[string]bool{}

	if opts.skipKnown("openlibrary") {
		unresolved["openlibrary"] = true
	} else if results, err := s.external.Search(ctx, query); err != nil {
		unresolved["openlibrary"] = true
		logger.WarnContext(ctx, "open library search failed",
			slog.String("title", logTitle), slog.Any("error", err))
	} else if m, ok := pick(olCandidates(results)); ok {
		out = append(out, newSourceProposalFromCandidate("openlibrary", m))
	}

	if s.googleBooks != nil {
		switch {
		case gbBreakerTripped(opts):
			unresolved["googlebooks"] = true
		case opts.skipKnown("googlebooks"):
			unresolved["googlebooks"] = true
		default:
			if results, err := s.googleBooks.Search(ctx, query); err != nil {
				unresolved["googlebooks"] = true
				if opts != nil && opts.gbExceeded != nil &&
					errors.Is(err, googlebooks.ErrRateLimited) {
					opts.gbExceeded.Store(true)
				}
				logger.WarnContext(ctx, "google books search failed",
					slog.String("title", logTitle), slog.Any("error", err))
			} else if m, ok := pick(gbCandidates(results)); ok {
				out = append(out, newSourceProposalFromCandidate("googlebooks", m))
			}
		}
	}

	if s.uniCat != nil && opts.skipKnown("unicat") {
		unresolved["unicat"] = true
	} else if s.uniCat != nil {
		if results, err := s.uniCat.Search(ctx, query); err != nil {
			unresolved["unicat"] = true
			logger.WarnContext(ctx, "unicat search failed",
				slog.String("title", logTitle), slog.Any("error", err))
		} else if m, ok := pick(ucCandidates(results)); ok {
			out = append(out, newSourceProposalFromCandidate("unicat", m))
		}
	}

	return out, unresolved
}

// fetchProposals routes between the standard guarded fetch and the override
// search used when an admin manually steers the query on an unmatched book.
// An override always uses the search path (even for ISBN'd books — the point
// is to escape the stored fields) and skips the match guards.
// ponytail: override takes each provider's top result unguarded — the admin
// reviews the full candidate before applying; tighten to guards-against-the-
// override-values if it misfires in practice.
func (s *BookService) fetchProposals(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	overrideTitle string,
	overrideAuthor string,
) []SourceProposal {
	// nil opts: this is the on-demand book-page path, not the bulk scan — it
	// must always query every configured provider fresh, never skip a
	// resolved source or trip the GB breaker.
	if overrideTitle == "" && overrideAuthor == "" {
		proposals, _ := s.fetchSourceProposals(ctx, logger, book, nil)
		return proposals
	}

	title := book.Title
	if overrideTitle != "" {
		title = overrideTitle
	}
	authors := book.Authors
	if overrideAuthor != "" {
		authors = []string{overrideAuthor}
	}

	proposals, _ := s.searchProviders(
		ctx, logger, title, buildSearchQuery(title, authors), firstCandidate, nil,
	)
	return proposals
}

func firstCandidate(
	candidates []titleOnlyCandidate,
) (titleOnlyCandidate, bool) {
	if len(candidates) == 0 {
		return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value intended
	}
	return candidates[0], true
}

// matchSearchResult picks the best-matching candidate from one provider's
// search results: the first title+author match when the book has authors
// (highest confidence), otherwise the ambiguity-guarded title-only match.
func matchSearchResult(
	book models.Book,
	candidates []titleOnlyCandidate,
) (titleOnlyCandidate, bool) {
	if len(book.Authors) > 0 {
		for _, c := range candidates {
			if titleAuthorMatch(book.Title, book.Authors, c.title, c.authors) {
				return c, true
			}
		}
		return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value intended
	}
	return selectTitleOnlyMatch(book.Title, candidates)
}

func olCandidates(results []openlibrary.ExternalBook) []titleOnlyCandidate {
	out := make([]titleOnlyCandidate, len(results))
	for i, r := range results {
		out[i] = titleOnlyCandidate{
			title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
			coverURL: r.CoverURL, description: r.Description, pageCount: r.PageCount,
		}
	}
	return out
}

func gbCandidates(results []googlebooks.ExternalBook) []titleOnlyCandidate {
	out := make([]titleOnlyCandidate, len(results))
	for i, r := range results {
		out[i] = titleOnlyCandidate{
			title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
			coverURL: r.CoverURL, description: r.Description, pageCount: r.PageCount,
		}
	}
	return out
}

func ucCandidates(results []unicat.ExternalBook) []titleOnlyCandidate {
	out := make([]titleOnlyCandidate, len(results))
	for i, r := range results {
		out[i] = titleOnlyCandidate{ //nolint:exhaustruct // UniCat has no cover images
			title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
			description: r.Description, pageCount: r.PageCount,
		}
	}
	return out
}

func newSourceProposalFromCandidate(
	source string,
	c titleOnlyCandidate,
) SourceProposal {
	p := SourceProposal{ //nolint:exhaustruct // Differs computed later, not stored
		Source:  source,
		Title:   c.title,
		Authors: c.authors,
	}
	if c.isbn13 != nil {
		p.ISBN13 = normalizeISBN(*c.isbn13)
	}
	if c.coverURL != nil {
		p.CoverURL = *c.coverURL
	}
	if c.description != nil {
		p.Description = *c.description
	}
	if c.pageCount != nil {
		p.PageCount = *c.pageCount
	}
	return p
}

// computeDifferences reports which fields of p differ from the library book.
// A field only counts as a difference when the source actually supplied a
// value: cover/isbn only flag when the library is missing that field (a
// source's cover/ISBN can't be judged "better" than an existing one), while
// title/authors/description/page_count flag on any non-empty mismatch —
// // ponytail: cover flagged only when library lacks one, no "better cover" guess.
func computeDifferences(book models.Book, p SourceProposal) []string {
	var diffs []string

	if p.Title != "" && normalizeTitle(p.Title) != normalizeTitle(book.Title) {
		diffs = append(diffs, "title")
	}
	if len(p.Authors) > 0 && !sameAuthorSet(book.Authors, p.Authors) {
		diffs = append(diffs, "authors")
	}
	if p.Description != "" &&
		normalizeString(p.Description) != normalizeString(derefStr(book.Description)) {
		diffs = append(diffs, "description")
	}
	if p.PageCount != 0 && (book.PageCount == nil || *book.PageCount != p.PageCount) {
		diffs = append(diffs, "page_count")
	}
	if p.ISBN13 != "" && (book.ISBN13 == nil || *book.ISBN13 == "") {
		diffs = append(diffs, "isbn13")
	}
	if p.CoverURL != "" && (book.CoverURL == nil || *book.CoverURL == "") {
		diffs = append(diffs, "cover_url")
	}

	return diffs
}

func sameAuthorSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[normalizeAuthor(x)] = struct{}{}
	}
	for _, x := range b {
		if _, ok := set[normalizeAuthor(x)]; !ok {
			return false
		}
	}
	return true
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// libraryProposal builds the Source == "" SourceProposal view of a catalog
// book's current values.
func libraryProposal(book models.Book) SourceProposal {
	p := SourceProposal{ //nolint:exhaustruct // Source "" is the library row
		Title:   book.Title,
		Authors: book.Authors,
	}
	if book.ISBN13 != nil {
		p.ISBN13 = *book.ISBN13
	}
	if book.CoverURL != nil {
		p.CoverURL = *book.CoverURL
	}
	if book.Description != nil {
		p.Description = *book.Description
	}
	if book.PageCount != nil {
		p.PageCount = *book.PageCount
	}
	return p
}

// ListResyncProposals returns every book flagged by the last BuildResyncProposals
// scan, with each source's Differs recomputed against the book's current
// library values (so an edit made outside the wizard between scan and review
// is reflected rather than shown stale).
func (s *BookService) ListResyncProposals(
	ctx context.Context,
) ([]ResyncProposal, error) {
	rows, err := s.resyncRepo().ListResyncProposals(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ResyncProposal, 0, len(rows))
	for _, row := range rows {
		proposal, decodeErr := decodeResyncProposalRow(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		out = append(out, proposal)
	}
	return out, nil
}

func decodeResyncProposalRow(
	row repositories.ResyncProposalRow,
) (ResyncProposal, error) {
	var sources []SourceProposal
	if err := json.Unmarshal(row.ProposalsJSON, &sources); err != nil {
		return ResyncProposal{}, fmt.Errorf(
			"decode resync proposals for book %s: %w", row.Book.ID, err,
		)
	}
	for i := range sources {
		sources[i].Differs = computeDifferences(row.Book, sources[i])
	}
	return ResyncProposal{
		BookID:  row.Book.ID.String(),
		Library: libraryProposal(row.Book),
		Sources: sources,
	}, nil
}

// ApplyResyncChoice resolves one book's pending proposal: source == "" keeps
// the library row unchanged (the proposal is simply dismissed); otherwise the
// chosen provider's fields are written onto the book. An existing ISBN13 is
// never overwritten, even if the chosen source disagrees — same rule the old
// resync path enforced.
func (s *BookService) ApplyResyncChoice(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	source string,
) error {
	row, err := s.resyncRepo().GetResyncProposal(ctx, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return ErrProposalNotFound
	}
	if err != nil {
		return err
	}

	if source != "" {
		if err = s.applyChosenSource(ctx, logger, *row, source); err != nil {
			return err
		}
	}

	return s.resyncRepo().DeleteResyncProposal(ctx, bookID)
}

func (s *BookService) applyChosenSource(
	ctx context.Context,
	logger *slog.Logger,
	row repositories.ResyncProposalRow,
	source string,
) error {
	var sources []SourceProposal
	if err := json.Unmarshal(row.ProposalsJSON, &sources); err != nil {
		return fmt.Errorf("decode resync proposals for book %s: %w", row.Book.ID, err)
	}

	return s.applySelectedSource(ctx, logger, row.Book, sources, source)
}

// applySelectedSource writes the chosen source's fields onto book. Shared by
// the stored-proposal path (ApplyResyncChoice) and the live per-book path
// (SyncBookSource). Returns ErrProposalNotFound if source isn't among sources.
func (s *BookService) applySelectedSource(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	sources []SourceProposal,
	source string,
) error {
	var chosen *SourceProposal
	for i := range sources {
		if sources[i].Source == source {
			chosen = &sources[i]
			break
		}
	}
	if chosen == nil {
		return ErrProposalNotFound
	}

	var coverURL, description, isbn13, title *string
	var pageCount *int
	if chosen.CoverURL != "" {
		coverURL = &chosen.CoverURL
	}
	if chosen.Description != "" {
		description = &chosen.Description
	}
	if chosen.Title != "" {
		title = &chosen.Title
	}
	if chosen.PageCount != 0 {
		pageCount = &chosen.PageCount
	}
	// Never overwrite an existing ISBN — only fill it in when the book has none.
	if chosen.ISBN13 != "" && (book.ISBN13 == nil || *book.ISBN13 == "") {
		isbn13 = &chosen.ISBN13
	}

	return s.writeResyncResult(
		ctx, logger, book,
		coverURL, description, pageCount, isbn13, title, chosen.Authors,
		chosen.Source,
	)
}

// GetBookSources fetches every configured provider's live view of one book
// for the admin book-page source selector — same fetch logic the wizard's
// scan uses (fetchSourceProposals), just for a single book on demand instead
// of the whole catalog.
func (s *BookService) GetBookSources(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	overrideTitle string,
	overrideAuthor string,
) (ResyncProposal, error) {
	book, err := s.resyncRepo().GetBookByID(ctx, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return ResyncProposal{}, ErrProposalNotFound
	}
	if err != nil {
		return ResyncProposal{}, err
	}

	sources := s.fetchProposals(ctx, logger, *book, overrideTitle, overrideAuthor)
	for i := range sources {
		sources[i].Differs = computeDifferences(*book, sources[i])
	}

	return ResyncProposal{
		BookID:  book.ID.String(),
		Library: libraryProposal(*book),
		Sources: sources,
	}, nil
}

// SyncBookSource live-fetches one book's sources and applies the chosen one —
// the book-page equivalent of ApplyResyncChoice, usable on any book without
// requiring a prior wizard scan to have flagged it. Also clears any pending
// wizard proposal for the book, since it's now resolved.
func (s *BookService) SyncBookSource(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	source string,
	overrideTitle string,
	overrideAuthor string,
) error {
	book, err := s.resyncRepo().GetBookByID(ctx, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return ErrProposalNotFound
	}
	if err != nil {
		return err
	}

	// "" keeps the library row unchanged — same as ApplyResyncChoice's dismiss.
	if source != "" {
		sources := s.fetchProposals(ctx, logger, *book, overrideTitle, overrideAuthor)
		if err = s.applySelectedSource(ctx, logger, *book, sources, source); err != nil {
			return err
		}
	}

	// Best-effort: dismiss any pending wizard proposal now that this book has
	// been resolved live. A missing proposal is not an error here.
	if err = s.resyncRepo().DeleteResyncProposal(ctx, bookID); err != nil &&
		!errors.Is(err, database.ErrResourceNotFound) {
		logger.WarnContext(
			ctx,
			"failed to clear pending resync proposal after live sync",
			slog.String("bookID", bookID.String()),
			slog.Any("error", err),
		)
	}

	return nil
}

// writeResyncResult persists the chosen fields and busts the cover cache when
// a new cover URL was resolved.
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
	metadataSource string,
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
		metadataSource,
	); dbErr != nil {
		return dbErr
	}

	if coverURL != nil {
		s.bustCoverCache(ctx, logger, book.ID)
	}

	return nil
}

// GetSourceStats reports per-source scan coverage and uniqueness over the
// whole catalog, for the admin source-stats report.
func (s *BookService) GetSourceStats(
	ctx context.Context,
) (*repositories.SourceStats, error) {
	return s.resyncRepo().GetSourceStats(ctx)
}

// ListBooksInExactSources returns the catalog books found by exactly the
// given set of sources, for drilling into a GetSourceStats unique_count (one
// source) or overlap combo (two or three sources).
func (s *BookService) ListBooksInExactSources(
	ctx context.Context,
	sources []string,
) ([]models.Book, error) {
	return s.resyncRepo().ListBooksInExactSources(ctx, sources)
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
		set := make(map[string]struct{}, len(m.authors))
		for _, a := range m.authors {
			if n := normalizeAuthor(a); n != "" {
				set[n] = struct{}{}
			}
		}
		authorSets[i] = set
	}

	for i := range authorSets {
		for j := i + 1; j < len(authorSets); j++ {
			if len(authorSets[i]) == 0 || len(authorSets[j]) == 0 {
				continue
			}
			if !setsOverlap(authorSets[i], authorSets[j]) {
				return titleOnlyCandidate{}, false //nolint:exhaustruct // zero value intended
			}
		}
	}

	return matching[0], true
}

func setsOverlap(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

// bustCoverCache deletes both the cached cover image and the negative-cache
// missing marker from R2 so that GetBookCover re-fetches on the next request.
func (s *BookService) bustCoverCache(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
) {
	if delErr := s.objectStore.Delete(ctx, bookCoverKey(bookID)); delErr != nil {
		logger.WarnContext(ctx, "failed to bust cover cache",
			slog.String("bookID", bookID.String()), slog.Any("error", delErr))
	}
	if delErr := s.objectStore.Delete(ctx, bookCoverMissingKey(bookID)); delErr != nil {
		logger.WarnContext(ctx, "failed to bust cover missing marker",
			slog.String("bookID", bookID.String()), slog.Any("error", delErr))
	}
}
