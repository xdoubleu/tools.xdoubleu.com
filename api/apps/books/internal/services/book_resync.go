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
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/authorname"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/database"
)

// ErrProposalNotFound is returned when the book has no pending proposal.
var ErrProposalNotFound = errors.New("resync proposal not found")

// ResyncSource is the subset of BooksRepository the resync path depends on.
type ResyncSource interface {
	ListCatalogBooks(ctx context.Context) ([]models.Book, error)
	GetBookByID(ctx context.Context, bookID uuid.UUID) (*models.Book, error)
	RefreshBookExternalData(
		ctx context.Context,
		bookID uuid.UUID,
		coverURL string,
		description string,
		pageCount int,
		isbn13 string,
		title string,
		authors []string,
		metadataSource string,
	) error
	UpdateResyncScanStatus(
		ctx context.Context,
		bookID uuid.UUID,
		uniCatFound *bool,
		hardcoverFound *bool,
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

// SourceProposal is one candidate metadata set for a catalog book: the library's
// current values (Source == "") or one provider's proposal.
type SourceProposal struct {
	Source      string   `json:"source"`
	CoverURL    string   `json:"cover_url,omitempty"`
	Description string   `json:"description,omitempty"`
	PageCount   int      `json:"page_count,omitempty"`
	ISBN13      string   `json:"isbn13,omitempty"`
	Title       string   `json:"title,omitempty"`
	Authors     []string `json:"authors,omitempty"`
	// Index is the 0-based ordinal among proposals from the same Source; nonzero
	// only for the manual override search (see topCandidates).
	Index int `json:"index,omitempty"`
	// Differs is computed at read time, never persisted.
	Differs []string `json:"-"`
}

// ResyncProposal pairs a catalog book with its differing source proposals.
type ResyncProposal struct {
	BookID  string
	Library SourceProposal
	Sources []SourceProposal
}

// BuildResyncProposals fetches every source independently for every catalog
// book and flags a book when a source is more complete than the library or no
// source found it (a coverage gap). Nothing is written to books; the proposals
// table is replaced wholesale. force bypasses the skip-if-known cache (see
// scanOptions). onProgress may be nil.
func (s *BookService) BuildResyncProposals(
	ctx context.Context,
	logger *slog.Logger,
	onProgress func(processed, total int),
	force bool,
) (int, error) {
	books, err := s.resyncSource.ListCatalogBooks(ctx)
	if err != nil {
		return 0, err
	}

	total := len(books)
	if onProgress != nil {
		onProgress(0, total)
	}

	// The client-side rate limiters are the real throttle; this bounds goroutines.
	const concurrency = 5

	//nolint:exhaustruct // errs/mu zero values fine
	acc := &resyncAccumulator{entries: make(map[uuid.UUID][]byte)}
	var processed atomic.Int64

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrency)

	for _, book := range books {
		b := book
		eg.Go(func() error {
			if egCtx.Err() != nil {
				return nil //nolint:nilerr // cancellation is not a failure
			}
			opts := &scanOptions{
				known: knownFor(b, force),
			}
			s.scanBookForResync(egCtx, logger, b, opts, acc)
			if onProgress != nil {
				onProgress(int(processed.Add(1)), total)
			}
			return nil
		})
	}
	_ = eg.Wait()

	// Cancelled mid-run: keep the old proposals table, since a partial scan would
	// erase proposals for books this run never reached.
	if ctx.Err() != nil {
		return len(acc.entries), nil //nolint:nilerr // cancellation is not a failure
	}

	if err = s.resyncSource.ReplaceResyncProposals(ctx, acc.entries); err != nil {
		return 0, err
	}

	return len(acc.entries), errors.Join(acc.errs...)
}

// resyncAccumulator collects per-book results from concurrent scans.
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
	s.ensureCoverCached(ctx, book)
}

// ensureCoverCached backfills the R2 cover for books with a CoverURL but no
// cached object. Best-effort: a cover miss must never fail the scan.
func (s *BookService) ensureCoverCached(ctx context.Context, book models.Book) {
	if book.CoverURL == nil || *book.CoverURL == "" {
		return
	}
	exists, err := s.objectStore.Exists(ctx, bookCoverKey(book.ID))
	if err != nil || exists {
		return
	}
	_ = s.cacheCoverFromURL(ctx, book.ID, *book.CoverURL)
}

// recordScanStatus persists per-source found flags. A nil flag leaves the
// column unchanged: the source wasn't resolved this pass (unconfigured,
// unsearchable, skipped as known, or errored).
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

	var ucFound, hcFound *bool
	if s.uniCat != nil {
		ucFound = found("unicat")
	}
	if s.hardcover != nil {
		hcFound = found("hardcover")
	}

	return s.resyncSource.UpdateResyncScanStatus(
		ctx, book.ID, ucFound, hcFound,
	)
}

// encodeIfFlagged returns the marshaled proposals and true when a source is
// strictly more complete than the book (applying replaces metadata wholesale,
// so a lateral source isn't worth it) or every queryable source came up empty.
// Unsearchable books are never flagged. (nil, true) means marshal failure.
func encodeIfFlagged(book models.Book, proposals []SourceProposal) ([]byte, bool) {
	attempted := (book.ISBN13 != nil && *book.ISBN13 != "") || book.Title != ""
	// An incremental scan skips already-found sources, so empty proposals must not
	// read as a gap for a book that's actually covered.
	notFoundAnywhere := attempted && len(proposals) == 0 && !anyKnownFound(book)
	if !notFoundAnywhere && !anySourceMoreComplete(book, proposals) {
		return nil, false
	}
	raw, err := json.Marshal(proposals)
	if err != nil {
		return nil, true
	}
	return raw, true
}

func anyKnownFound(book models.Book) bool {
	isTrue := func(b *bool) bool { return b != nil && *b }
	return isTrue(book.UniCatFound) ||
		isTrue(book.HardcoverFound)
}

func bookFieldCount(book models.Book) int {
	count := 0
	if book.Title != "" {
		count++
	}
	if len(book.Authors) > 0 {
		count++
	}
	if book.Description != nil && *book.Description != "" {
		count++
	}
	if book.PageCount != nil && *book.PageCount != 0 {
		count++
	}
	if book.ISBN13 != nil && *book.ISBN13 != "" {
		count++
	}
	if book.CoverURL != nil && *book.CoverURL != "" {
		count++
	}
	return count
}

func proposalFieldCount(p SourceProposal) int {
	count := 0
	if p.Title != "" {
		count++
	}
	if len(p.Authors) > 0 {
		count++
	}
	if p.Description != "" {
		count++
	}
	if p.PageCount != 0 {
		count++
	}
	if p.ISBN13 != "" {
		count++
	}
	if p.CoverURL != "" {
		count++
	}
	return count
}

// anySourceMoreComplete reports whether any source supplies strictly more
// comparable fields than the book has.
func anySourceMoreComplete(book models.Book, proposals []SourceProposal) bool {
	current := bookFieldCount(book)
	for _, p := range proposals {
		if proposalFieldCount(p) > current {
			return true
		}
	}
	return false
}

// scanOptions gates the bulk scan. known skips sources already resolved (true
// or false) for the book, so found columns act as a durable cache; force
// leaves it empty. nil means on-demand mode: query every source fresh.
type scanOptions struct {
	known map[string]bool
}

func (opts *scanOptions) skipKnown(source string) bool {
	return opts != nil && opts.known[source]
}

// knownFor returns the sources already resolved for this book, or an empty
// set when force bypasses the cache.
func knownFor(book models.Book, force bool) map[string]bool {
	known := map[string]bool{}
	if force {
		return known
	}
	if book.UniCatFound != nil {
		known["unicat"] = true
	}
	if book.HardcoverFound != nil {
		known["hardcover"] = true
	}
	return known
}

// fetchSourceProposals fetches each provider's view of one book independently:
// by ISBN13 when present, otherwise a guarded title/author search. The second
// result names sources not resolved this pass (see recordScanStatus).
func (s *BookService) fetchSourceProposals(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	if book.ISBN13 != nil && *book.ISBN13 != "" {
		return s.fetchByISBN(ctx, logger, book, opts)
	}
	if book.Title == "" {
		return nil, nil
	}
	return s.fetchBySearch(ctx, logger, book, opts)
}

// fetchByISBN queries each provider by ISBN independently; Hardcover and UniCat
// fall back to a guarded title+author search on a miss because their ISBN
// indexes miss books their title indexes have.
func (s *BookService) fetchByISBN(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	unresolved := map[string]bool{}

	var ucProposal *SourceProposal
	var ucUnresolved bool
	var hcProposal *SourceProposal
	var hcUnresolved bool

	eg, egCtx := errgroup.WithContext(ctx)

	if s.uniCat != nil {
		eg.Go(func() error {
			p, unres := s.fetchUniCatByISBN(egCtx, logger, book, opts)
			ucProposal, ucUnresolved = p, unres
			return nil
		})
	}

	if s.hardcover != nil {
		eg.Go(func() error {
			p, unres := s.fetchHardcoverByISBN(egCtx, logger, book, opts)
			hcProposal, hcUnresolved = p, unres
			return nil
		})
	}

	_ = eg.Wait()

	var out []SourceProposal
	if s.uniCat != nil {
		if ucUnresolved {
			unresolved["unicat"] = true
		} else if ucProposal != nil {
			out = append(out, *ucProposal)
		}
	}
	if s.hardcover != nil {
		if hcUnresolved {
			unresolved["hardcover"] = true
		} else if hcProposal != nil {
			out = append(out, *hcProposal)
		}
	}

	return out, unresolved
}

// fetchUniCatByISBN queries UniCat by ISBN (gated only by skip-if-known),
// falling back to a guarded search: its ISBN index (020$a) reflects only the
// physical item catalogued.
func (s *BookService) fetchUniCatByISBN(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) (*SourceProposal, bool) {
	if opts.skipKnown("unicat") {
		return nil, true
	}

	isbn13 := *book.ISBN13
	ucDetail, ucErr := s.uniCat.GetByISBN(ctx, isbn13)
	if ucErr != nil && !errors.Is(ucErr, unicat.ErrNotFound) {
		logger.WarnContext(ctx, "unicat ISBN lookup failed",
			slog.String("isbn13", isbn13), slog.Any("error", ucErr))
		return nil, true
	}
	if ucDetail == nil {
		return s.fetchUniCatBySearchFallback(ctx, logger, book)
	}

	p := newSourceProposalFromCandidate(
		"unicat",
		titleOnlyCandidate{ //nolint:exhaustruct // UniCat has no cover images
			title:       ucDetail.Title,
			authors:     ucDetail.Authors,
			isbn13:      ucDetail.ISBN13,
			description: ucDetail.Description,
			pageCount:   ucDetail.PageCount,
		},
	)
	return &p, false
}

func (s *BookService) fetchUniCatBySearchFallback(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
) (*SourceProposal, bool) {
	if book.Title == "" {
		return nil, false
	}

	results, err := s.uniCat.Search(
		ctx, buildSearchQuery(book.Title, book.Authors),
	)
	if err != nil {
		logger.WarnContext(ctx, "unicat search fallback failed",
			slog.String("title", book.Title), slog.Any("error", err))
		return nil, true
	}

	m, ok := matchSearchResult(book, ucCandidates(results))
	if !ok {
		return nil, false
	}
	p := newSourceProposalFromCandidate("unicat", m)
	return &p, false
}

// fetchHardcoverByISBN queries Hardcover by ISBN (gated only by
// skip-if-known), falling back to a guarded search: its edition-level ISBN
// coverage is sparse while its Typesense work index is comprehensive.
func (s *BookService) fetchHardcoverByISBN(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) (*SourceProposal, bool) {
	if opts.skipKnown("hardcover") {
		return nil, true
	}

	isbn13 := *book.ISBN13
	hcDetail, hcErr := s.hardcover.GetByISBN(ctx, isbn13)
	if hcErr != nil && !errors.Is(hcErr, hardcover.ErrNotFound) {
		logger.WarnContext(ctx, "hardcover ISBN lookup failed",
			slog.String("isbn13", isbn13), slog.Any("error", hcErr))
		return nil, true
	}
	if hcDetail == nil {
		return s.fetchHardcoverBySearchFallback(ctx, logger, book)
	}

	p := newSourceProposalFromCandidate("hardcover", titleOnlyCandidate{
		title: hcDetail.Title, authors: hcDetail.Authors, isbn13: hcDetail.ISBN13,
		coverURL: hcDetail.CoverURL, description: hcDetail.Description,
		pageCount: hcDetail.PageCount,
	})
	return &p, false
}

func (s *BookService) fetchHardcoverBySearchFallback(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
) (*SourceProposal, bool) {
	if book.Title == "" {
		return nil, false
	}

	results, err := s.hardcover.Search(
		ctx, buildSearchQuery(book.Title, book.Authors),
	)
	if err != nil {
		logger.WarnContext(ctx, "hardcover search fallback failed",
			slog.String("title", book.Title), slog.Any("error", err))
		return nil, true
	}

	m, ok := matchSearchResult(book, hcCandidates(results))
	if !ok {
		return nil, false
	}
	p := newSourceProposalFromCandidate("hardcover", m)
	return &p, false
}

// fetchBySearch keeps the first guarded match per provider (titleAuthorMatch,
// or selectTitleOnlyMatch when the book has no authors).
func (s *BookService) fetchBySearch(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	return s.searchProviders(
		ctx, logger, book.Title, buildSearchQuery(book.Title, book.Authors),
		book.Authors,
		single(func(candidates []titleOnlyCandidate) (titleOnlyCandidate, bool) {
			return matchSearchResult(book, candidates)
		}),
		opts,
	)
}

// single adapts a one-candidate picker to the multi-candidate picker shape.
func single(
	pick func([]titleOnlyCandidate) (titleOnlyCandidate, bool),
) func([]titleOnlyCandidate) []titleOnlyCandidate {
	return func(candidates []titleOnlyCandidate) []titleOnlyCandidate {
		if m, ok := pick(candidates); ok {
			return []titleOnlyCandidate{m}
		}
		return nil
	}
}

// topCandidates keeps the first n candidates unguarded, for the manual
// override search.
func topCandidates(n int) func([]titleOnlyCandidate) []titleOnlyCandidate {
	return func(candidates []titleOnlyCandidate) []titleOnlyCandidate {
		if len(candidates) > n {
			return candidates[:n]
		}
		return candidates
	}
}

// searchProviders queries every provider with one query and keeps what pick
// selects. authors filters Hardcover only: its Typesense query is title-only
// and author-blind, while UniCat filters server-side via inauthor:.
//
//nolint:gocognit // two independent concurrent source searches
func (s *BookService) searchProviders(
	ctx context.Context,
	logger *slog.Logger,
	logTitle string,
	query string,
	authors []string,
	pick func([]titleOnlyCandidate) []titleOnlyCandidate,
	opts *scanOptions,
) ([]SourceProposal, map[string]bool) {
	unresolved := map[string]bool{}

	var ucPicked []titleOnlyCandidate
	var ucUnresolved bool
	var hcPicked []titleOnlyCandidate
	var hcUnresolved bool

	eg, egCtx := errgroup.WithContext(ctx)

	if s.uniCat != nil {
		eg.Go(func() error {
			if opts.skipKnown("unicat") {
				ucUnresolved = true
				return nil
			}
			results, err := s.uniCat.Search(egCtx, query)
			if err != nil {
				ucUnresolved = true
				logger.WarnContext(egCtx, "unicat search failed",
					slog.String("title", logTitle), slog.Any("error", err))
				return nil
			}
			ucPicked = pick(ucCandidates(results))
			return nil
		})
	}

	if s.hardcover != nil {
		eg.Go(func() error {
			if opts.skipKnown("hardcover") {
				hcUnresolved = true
				return nil
			}
			results, err := s.hardcover.Search(egCtx, query)
			if err != nil {
				hcUnresolved = true
				logger.WarnContext(egCtx, "hardcover search failed",
					slog.String("title", logTitle), slog.Any("error", err))
				return nil
			}
			hcPicked = pick(filterByAuthor(hcCandidates(results), authors))
			return nil
		})
	}

	_ = eg.Wait()

	var out []SourceProposal
	if s.uniCat != nil {
		if ucUnresolved {
			unresolved["unicat"] = true
		} else {
			out = appendPicked(out, "unicat", ucPicked)
		}
	}
	if s.hardcover != nil {
		if hcUnresolved {
			unresolved["hardcover"] = true
		} else {
			out = appendPicked(out, "hardcover", hcPicked)
		}
	}

	return out, unresolved
}

// fetchProposals is the on-demand book-page path. It always matches by
// title+author search, even with an ISBN, so the candidate set stays stable
// across repeated applies. An override takes each provider's top N unguarded
// (author still filtered) for the admin to review.
const overrideMaxCandidates = 5

func (s *BookService) fetchProposals(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	overrideTitle string,
	overrideAuthor string,
) []SourceProposal {
	// On-demand: query every provider fresh, never skip or trip the breaker.
	if overrideTitle == "" && overrideAuthor == "" {
		if book.Title == "" {
			return nil
		}
		proposals, _ := s.fetchBySearch(ctx, logger, book, nil)
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
		ctx, logger, title, buildSearchQuery(title, authors), authors,
		topCandidates(overrideMaxCandidates), nil,
	)
	return proposals
}

// matchSearchResult picks the first title+author match when the book has
// authors, otherwise the ambiguity-guarded title-only match.
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

// filterByAuthor keeps candidates sharing a normalised author last name with
// authors; with no usable authors, all candidates pass.
func filterByAuthor(
	candidates []titleOnlyCandidate,
	authors []string,
) []titleOnlyCandidate {
	lastNames := make(map[string]struct{}, len(authors))
	for _, a := range authors {
		if n := normalizeAuthor(a); n != "" {
			lastNames[n] = struct{}{}
		}
	}
	if len(lastNames) == 0 {
		return candidates
	}

	var out []titleOnlyCandidate
	for _, c := range candidates {
		for _, a := range c.authors {
			if _, ok := lastNames[normalizeAuthor(a)]; ok {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func hcCandidates(results []hardcover.ExternalBook) []titleOnlyCandidate {
	out := make([]titleOnlyCandidate, len(results))
	for i, r := range results {
		out[i] = titleOnlyCandidate{
			title: r.Title, authors: r.Authors, isbn13: r.ISBN13,
			coverURL: r.CoverURL, description: r.Description, pageCount: r.PageCount,
		}
	}
	return out
}

func appendPicked(
	out []SourceProposal,
	source string,
	picked []titleOnlyCandidate,
) []SourceProposal {
	return append(out, newSourceProposalsFromCandidates(source, picked)...)
}

func newSourceProposalsFromCandidates(
	source string,
	candidates []titleOnlyCandidate,
) []SourceProposal {
	out := make([]SourceProposal, len(candidates))
	for i, c := range candidates {
		p := newSourceProposalFromCandidate(source, c)
		p.Index = i
		out[i] = p
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
// Only supplied values count; cover/isbn flag only when the library lacks one.
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

// ListResyncProposals returns the flagged books with Differs recomputed
// against current library values.
func (s *BookService) ListResyncProposals(
	ctx context.Context,
) ([]ResyncProposal, error) {
	rows, err := s.resyncSource.ListResyncProposals(ctx)
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

// ApplyResyncChoice resolves one book's pending proposal: source == "" just
// dismisses it; otherwise the chosen provider's fields are written.
func (s *BookService) ApplyResyncChoice(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	source string,
) error {
	row, err := s.resyncSource.GetResyncProposal(ctx, bookID)
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

	return s.resyncSource.DeleteResyncProposal(ctx, bookID)
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

	return s.applySelectedSource(ctx, logger, row.Book, sources, source, 0)
}

// applySelectedSource replaces the book's metadata wholesale with the chosen
// source's; isbn13 is never blanked. index relies on the provider returning
// the same order on re-fetch as when the admin saw the candidates.
func (s *BookService) applySelectedSource(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	sources []SourceProposal,
	source string,
	index int,
) error {
	var chosen *SourceProposal
	for i := range sources {
		if sources[i].Source == source && sources[i].Index == index {
			chosen = &sources[i]
			break
		}
	}
	if chosen == nil {
		return ErrProposalNotFound
	}

	return s.writeResyncResult(
		ctx, logger, book,
		chosen.CoverURL, chosen.Description, chosen.PageCount, chosen.ISBN13,
		chosen.Title, chosen.Authors, chosen.Source,
	)
}

// GetBookSources fetches every provider's live view of one book for the admin
// source selector.
func (s *BookService) GetBookSources(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	overrideTitle string,
	overrideAuthor string,
) (ResyncProposal, error) {
	book, err := s.resyncSource.GetBookByID(ctx, bookID)
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

// SyncBookSource live-fetches one book's sources, applies the chosen one, and
// clears any pending wizard proposal.
func (s *BookService) SyncBookSource(
	ctx context.Context,
	logger *slog.Logger,
	bookID uuid.UUID,
	source string,
	index int,
	overrideTitle string,
	overrideAuthor string,
) error {
	book, err := s.resyncSource.GetBookByID(ctx, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return ErrProposalNotFound
	}
	if err != nil {
		return err
	}

	if source != "" {
		sources := s.fetchProposals(ctx, logger, *book, overrideTitle, overrideAuthor)
		err = s.applySelectedSource(ctx, logger, *book, sources, source, index)
		if err != nil {
			return err
		}
	}

	if err = s.resyncSource.DeleteResyncProposal(ctx, bookID); err != nil &&
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

// writeResyncResult persists the fields and refreshes the R2 cover when the
// cover URL changes (including to blank).
func (s *BookService) writeResyncResult(
	ctx context.Context,
	logger *slog.Logger,
	book models.Book,
	coverURL string,
	description string,
	pageCount int,
	isbn13 string,
	title string,
	authors []string,
	metadataSource string,
) error {
	authors = authorname.NormalizeAll(authors)

	if dbErr := s.resyncSource.RefreshBookExternalData(
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

	if coverURL == derefStr(book.CoverURL) {
		return nil
	}

	if coverURL == "" {
		if clearErr := s.clearCoverCache(ctx, book.ID); clearErr != nil {
			logger.WarnContext(ctx, "failed to clear book cover cache",
				slog.String("bookID", book.ID.String()), slog.Any("error", clearErr))
		}
		return nil
	}

	if cacheErr := s.cacheCoverFromURL(ctx, book.ID, coverURL); cacheErr != nil {
		logger.WarnContext(ctx, "failed to cache book cover",
			slog.String("bookID", book.ID.String()), slog.Any("error", cacheErr))
	}
	return nil
}

// GetSourceStats reports per-source scan coverage and uniqueness.
func (s *BookService) GetSourceStats(
	ctx context.Context,
) (*repositories.SourceStats, error) {
	return s.resyncSource.GetSourceStats(ctx)
}

// ListBooksInExactSources returns the books found by exactly the given sources.
func (s *BookService) ListBooksInExactSources(
	ctx context.Context,
	sources []string,
) ([]models.Book, error) {
	return s.resyncSource.ListBooksInExactSources(ctx, sources)
}

func buildSearchQuery(title string, authors []string) string {
	author := ""
	if len(authors) > 0 {
		author = authors[0]
	}
	if author == "" {
		return fmt.Sprintf("intitle:%q", title)
	}
	return fmt.Sprintf("intitle:%q inauthor:%q", title, author)
}

// titleAuthorMatch reports whether normalised titles match and at least one
// author last name overlaps. False when either title normalises to "".
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

	bookLastNames := make(map[string]struct{}, len(bookAuthors))
	for _, a := range bookAuthors {
		if n := normalizeAuthor(a); n != "" {
			bookLastNames[n] = struct{}{}
		}
	}
	if len(bookLastNames) == 0 {
		return false
	}

	for _, a := range resultAuthors {
		if n := normalizeAuthor(a); n != "" {
			if _, ok := bookLastNames[n]; ok {
				return true
			}
		}
	}
	return false
}

// titleOnlyCandidate is the provider-neutral search result shape.
type titleOnlyCandidate struct {
	title       string
	authors     []string
	isbn13      *string
	coverURL    *string
	description *string
	pageCount   *int
}

// selectTitleOnlyMatch returns the first title match, unless two matches have
// fully disjoint author sets (likely different books), in which case none.
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

	// A pair with no common author means two different books share the title.
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
