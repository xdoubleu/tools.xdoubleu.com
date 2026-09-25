package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/authorname"
	"tools.xdoubleu.com/apps/books/pkg/books"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/database"
)

// ErrExternalNotFound is returned when the provider is unavailable or has no match.
var ErrExternalNotFound = errors.New("external book not found")

const externalSearchMaxCandidates = 10

// Source names; "manual" marks hand-entered books with no provenance.
const (
	sourceHardcover = "hardcover"
	sourceUniCat    = "unicat"
	sourceManual    = "manual"
)

type BookService struct {
	logger       *slog.Logger
	books        *repositories.BooksRepository
	bookFiles    *repositories.BookFilesRepository
	objectStore  objectstore.Client
	readingState *repositories.BookReadingStateRepository
	uniCat       unicat.Client
	hardcover    hardcover.Client
	resyncSource ResyncSource
}

// SearchLibrary searches the user's own library by title/author substring.
func (s *BookService) SearchLibrary(
	ctx context.Context,
	userID string,
	query string,
	limit int32,
	offset int32,
) ([]models.UserBook, bool, error) {
	return s.books.SearchLibrary(ctx, userID, query, limit, offset)
}

// SearchExternal fans out to every configured provider and keeps each source's
// matches side by side. Interactive callers should narrow with
// FilterExternalByQuery; tryExternalLookup wants the providers' raw top hit.
func (s *BookService) SearchExternal(
	ctx context.Context,
	query string,
) []SourceProposal {
	if query == "" {
		return nil
	}
	proposals, _ := s.searchProviders(
		ctx, s.logger, query, buildSearchQuery(query, nil), nil,
		topCandidates(externalSearchMaxCandidates), nil,
	)
	return proposals
}

// FilterExternalByQuery keeps proposals whose title+authors contain every
// normalised query word (providers only search title). Unfiltered when query
// has no tokens.
func FilterExternalByQuery(query string, proposals []SourceProposal) []SourceProposal {
	tokens := titleTokens(query)
	if len(tokens) == 0 {
		return proposals
	}

	out := make([]SourceProposal, 0, len(proposals))
	for _, p := range proposals {
		haystack := normalizeString(p.Title + " " + strings.Join(p.Authors, " "))
		if containsAllTokens(haystack, tokens) {
			out = append(out, p)
		}
	}
	return out
}

func containsAllTokens(haystack string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

// GetExternal fetches one book from a provider by ISBN13 (the provider-scoped
// ID), returning ErrExternalNotFound when unavailable or unmatched.
func (s *BookService) GetExternal(
	ctx context.Context,
	provider string,
	providerID string,
) (*SourceProposal, error) {
	switch provider {
	case sourceHardcover:
		if s.hardcover == nil {
			return nil, ErrExternalNotFound
		}
		d, err := s.hardcover.GetByISBN(ctx, providerID)
		if err != nil {
			if errors.Is(err, hardcover.ErrNotFound) {
				return nil, ErrExternalNotFound
			}
			return nil, err
		}
		p := newSourceProposalFromCandidate(sourceHardcover, titleOnlyCandidate{
			title: d.Title, authors: d.Authors, isbn13: d.ISBN13,
			coverURL: d.CoverURL, description: d.Description, pageCount: d.PageCount,
		})
		return &p, nil
	case sourceUniCat:
		if s.uniCat == nil {
			return nil, ErrExternalNotFound
		}
		d, err := s.uniCat.GetByISBN(ctx, providerID)
		if err != nil {
			if errors.Is(err, unicat.ErrNotFound) {
				return nil, ErrExternalNotFound
			}
			return nil, err
		}
		p := newSourceProposalFromCandidate(
			sourceUniCat,
			titleOnlyCandidate{ //nolint:exhaustruct // UniCat has no cover images
				title: d.Title, authors: d.Authors, isbn13: d.ISBN13,
				description: d.Description, pageCount: d.PageCount,
			},
		)
		return &p, nil
	default:
		return nil, ErrExternalNotFound
	}
}

// SetBookISBN sets a catalog book's isbn13, returning ErrResourceConflict when
// another row already holds it.
func (s *BookService) SetBookISBN(
	ctx context.Context,
	bookID uuid.UUID,
	isbn13 string,
) error {
	// Normalize so hyphenated input hits the same unique index entry.
	isbn13 = normalizeISBN(isbn13)

	book, err := s.books.GetBookByID(ctx, bookID)
	if err != nil {
		return err
	}

	if conflictErr := s.checkISBNConflict(ctx, bookID, isbn13); conflictErr != nil {
		return conflictErr
	}

	book.ISBN13 = &isbn13
	return s.books.UpdateBookByID(ctx, *book)
}

func (s *BookService) checkISBNConflict(
	ctx context.Context,
	bookID uuid.UUID,
	isbn13 string,
) error {
	existing, err := s.books.GetCatalogBookByISBN13(ctx, isbn13)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}
	if existing != nil && existing.ID != bookID {
		return database.ErrResourceConflict
	}
	return nil
}

// UpdateBook overwrites a catalog book's metadata and syncs the cover cache to
// rawCoverURL (cleared when empty). Returns ErrResourceConflict on a taken ISBN.
func (s *BookService) UpdateBook(
	ctx context.Context,
	bookID uuid.UUID,
	metadata models.Book,
	rawCoverURL string,
) (*models.Book, error) {
	if _, err := s.books.GetBookByID(ctx, bookID); err != nil {
		return nil, err
	}

	if metadata.ISBN13 != nil {
		if err := s.checkISBNConflict(ctx, bookID, *metadata.ISBN13); err != nil {
			return nil, err
		}
	}

	metadata.ID = bookID
	if err := s.books.UpdateBookByID(ctx, metadata); err != nil {
		return nil, err
	}

	if rawCoverURL != "" {
		if cacheErr := s.cacheCoverFromURL(ctx, bookID, rawCoverURL); cacheErr != nil {
			s.logger.Warn("failed to cache book cover",
				"bookID", bookID, "err", cacheErr)
		}
	} else if clearErr := s.clearCoverCache(ctx, bookID); clearErr != nil {
		s.logger.Warn("failed to clear book cover cache",
			"bookID", bookID, "err", clearErr)
	}

	return s.books.GetBookByID(ctx, bookID)
}

func (s *BookService) AddToLibrary(
	ctx context.Context,
	userID string,
	ext SourceProposal,
	status string,
	initialTags []string,
) (*models.UserBook, error) {
	book := externalToBook(s.enrichByISBN(ctx, ext))
	saved, err := s.books.UpsertBook(ctx, book)
	if err != nil {
		return nil, err
	}

	// Eager-fetch into R2 so the cover proxy never needs a live fetch.
	if book.CoverURL != nil && *book.CoverURL != "" {
		if cacheErr := s.cacheCoverFromURL(ctx, saved.ID, *book.CoverURL); cacheErr != nil {
			s.logger.WarnContext(ctx, "failed to cache book cover",
				"bookID", saved.ID, "error", cacheErr)
		}
	}

	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:         userID,
		BookID:         saved.ID,
		Status:         status,
		Tags:           initialTags,
		ShelfPositions: map[string]int{},
	}
	if err = s.books.UpsertUserBook(ctx, ub); err != nil {
		return nil, err
	}
	if err = s.registerCustomShelf(ctx, userID, status); err != nil {
		return nil, err
	}

	return s.books.GetUserBook(ctx, userID, saved.ID)
}

func (s *BookService) UpdateStatus(
	ctx context.Context,
	userID string,
	ub models.UserBook,
) error {
	if err := s.books.UpsertUserBook(ctx, ub); err != nil {
		return err
	}
	return s.registerCustomShelf(ctx, userID, ub.Status)
}

// registerCustomShelf records a custom status so the shelf persists after its
// last book leaves. Built-in statuses are never stored.
func (s *BookService) registerCustomShelf(
	ctx context.Context,
	userID, status string,
) error {
	if builtInStatuses[status] {
		return nil
	}
	return s.books.EnsureShelf(ctx, userID, status)
}

// UpdateFinishedAt overwrites the read-date history for a user's book.
func (s *BookService) UpdateFinishedAt(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	finishedAt []time.Time,
) error {
	return s.books.UpdateFinishedAt(ctx, userID, bookID, finishedAt)
}

// ToggleTag adds or removes a tag from a user_book atomically.
func (s *BookService) ToggleTag(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	tag string,
) error {
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return fmt.Errorf("book not found")
		}
		return err
	}

	newTags := make([]string, 0, len(ub.Tags))
	found := false
	for _, t := range ub.Tags {
		if t == tag {
			found = true
			continue
		}
		newTags = append(newTags, t)
	}
	if !found {
		newTags = append(newTags, tag)
	}

	koboSyncEnabled := slices.Contains(newTags, models.TagKoboSync)
	if updateErr := s.books.UpdateTags(
		ctx, userID, bookID, newTags, koboSyncEnabled,
	); updateErr != nil {
		return updateErr
	}

	if tag != models.TagKoboSync {
		return nil
	}
	// Disabling leaves the copy on the device, so tombstone it; re-enabling clears
	// a stale tombstone.
	if koboSyncEnabled {
		return s.books.DeleteKoboRemoval(ctx, userID, bookID)
	}
	return s.books.UpsertKoboRemoval(ctx, userID, bookID)
}

func (s *BookService) GetUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.UserBook, error) {
	return s.books.GetUserBook(ctx, userID, bookID)
}

// builtInStatuses carry semantics (progress gating, rating unlock), so shelf
// RPCs can't rename or delete them.
//
//nolint:gochecknoglobals // effectively a constant set
var builtInStatuses = map[string]bool{
	models.StatusToRead:  true,
	models.StatusReading: true,
	models.StatusRead:    true,
	models.StatusDropped: true,
}

// ListShelves returns every registered custom shelf, including empty ones.
func (s *BookService) ListShelves(
	ctx context.Context,
	userID string,
) ([]string, error) {
	return s.books.ListShelves(ctx, userID)
}

// CreateShelf registers a new empty custom shelf.
func (s *BookService) CreateShelf(
	ctx context.Context,
	userID string,
	name string,
) error {
	if name == "" {
		return fmt.Errorf("shelf name cannot be empty")
	}
	if builtInStatuses[name] {
		return fmt.Errorf("cannot create built-in shelf %q", name)
	}
	return s.books.EnsureShelf(ctx, userID, name)
}

// RenameShelf renames a custom shelf (= status) across the user's library.
func (s *BookService) RenameShelf(
	ctx context.Context,
	userID string,
	oldName string,
	newName string,
) (uint32, error) {
	if builtInStatuses[oldName] {
		return 0, fmt.Errorf("cannot rename built-in shelf %q", oldName)
	}
	if builtInStatuses[newName] {
		return 0, fmt.Errorf("cannot rename shelf to built-in value %q", newName)
	}
	if newName == "" {
		return 0, fmt.Errorf("shelf name cannot be empty")
	}
	return s.books.RenameShelf(ctx, userID, oldName, newName)
}

// DeleteShelf moves every book on a custom shelf to targetName, which may be
// a built-in status.
func (s *BookService) DeleteShelf(
	ctx context.Context,
	userID string,
	name string,
	targetName string,
) (uint32, error) {
	if builtInStatuses[name] {
		return 0, fmt.Errorf("cannot delete built-in shelf %q", name)
	}
	if targetName == "" {
		return 0, fmt.Errorf("target shelf name cannot be empty")
	}
	return s.books.DeleteShelf(ctx, userID, name, targetName)
}

// RenameTag renames a tag across the user's library.
func (s *BookService) RenameTag(
	ctx context.Context,
	userID string,
	oldName string,
	newName string,
) (uint32, error) {
	if oldName == "" || newName == "" {
		return 0, fmt.Errorf("tag name cannot be empty")
	}
	return s.books.RenameTag(ctx, userID, oldName, newName)
}

// DeleteTag removes a tag from every book in the user's library.
func (s *BookService) DeleteTag(
	ctx context.Context,
	userID string,
	name string,
) (uint32, error) {
	if name == "" {
		return 0, fmt.Errorf("tag name cannot be empty")
	}
	return s.books.DeleteTag(ctx, userID, name)
}

func (s *BookService) GetByStatus(
	ctx context.Context,
	userID string,
	status string,
) ([]models.UserBook, error) {
	return s.books.GetByStatus(ctx, userID, status)
}

func (s *BookService) GetLibrary(
	ctx context.Context,
	userID string,
) ([]models.UserBook, error) {
	return s.books.GetLibrary(ctx, userID)
}

// ImportFromCSV upserts a Goodreads CSV export and returns the imported count.
func (s *BookService) ImportFromCSV(
	ctx context.Context,
	userID string,
	r io.Reader,
) (int, error) {
	entries, err := books.ParseCSV(r)
	if err != nil {
		return 0, err
	}

	bookList := make([]models.Book, len(entries))
	ubList := make([]models.UserBook, len(entries))
	for i, e := range entries {
		bookList[i] = e.Book
		bookList[i].Authors = authorname.NormalizeAll(e.Book.Authors)
		ubList[i] = e.UserBook
		ubList[i].UserID = userID
	}

	s.logger.DebugContext(ctx, fmt.Sprintf("importing %d books from CSV", len(entries)))

	if err = s.books.BatchUpsert(ctx, userID, bookList, ubList); err != nil {
		return 0, err
	}

	return len(entries), nil
}

// BuildReadProgress returns sorted cumulative labels+values for the progress chart.
func (s *BookService) BuildReadProgress(
	ctx context.Context,
	userID string,
) ([]string, []string, error) {
	dates, err := s.books.GetFinishedDates(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	uniqueDates := []string{}
	for _, d := range dates {
		ds := d.Format(models.ProgressDateFormat)
		if !slices.Contains(uniqueDates, ds) {
			uniqueDates = append(uniqueDates, ds)
		}
	}
	slices.Sort(uniqueDates)

	labels := make([]string, 0, len(uniqueDates))
	values := make([]string, 0, len(uniqueDates))
	cumulative := 0
	for _, ds := range uniqueDates {
		count := countDatesOn(dates, ds)
		cumulative += count
		labels = append(labels, ds)
		values = append(values, fmt.Sprintf("%d", cumulative))
	}

	return labels, values, nil
}

func countDatesOn(dates []time.Time, dateStr string) int {
	count := 0
	for _, d := range dates {
		if d.Format(models.ProgressDateFormat) == dateStr {
			count++
		}
	}
	return count
}

// enrichByISBN best-effort fills missing fields on a search result via the
// other providers' ISBN lookup, which can be more complete than search
// (UniCat especially). Failures never block an add.
func (s *BookService) enrichByISBN(
	ctx context.Context,
	ext SourceProposal,
) SourceProposal {
	if ext.ISBN13 == "" {
		return ext
	}
	if ext.Description != "" && ext.PageCount != 0 && ext.CoverURL != "" {
		return ext
	}

	if s.hardcover != nil {
		detail, err := s.hardcover.GetByISBN(ctx, ext.ISBN13)
		if err != nil && !errors.Is(err, hardcover.ErrNotFound) {
			s.logger.WarnContext(ctx, "hardcover ISBN lookup failed", "error", err)
		}
		if detail != nil {
			ext.Description = fillStrIfEmpty(ext.Description, detail.Description)
			ext.PageCount = fillIntIfZero(ext.PageCount, detail.PageCount)
			ext.CoverURL = fillStrIfEmpty(ext.CoverURL, detail.CoverURL)
		}
	}

	if s.uniCat != nil && (ext.Description == "" || ext.PageCount == 0) {
		detail, err := s.uniCat.GetByISBN(ctx, ext.ISBN13)
		if err != nil && !errors.Is(err, unicat.ErrNotFound) {
			s.logger.WarnContext(ctx, "unicat ISBN lookup failed", "error", err)
		}
		if detail != nil {
			ext.Description = fillStrIfEmpty(ext.Description, detail.Description)
			ext.PageCount = fillIntIfZero(ext.PageCount, detail.PageCount)
		}
	}

	return ext
}

func fillStrIfEmpty(cur string, src *string) string {
	if cur == "" && src != nil {
		return *src
	}
	return cur
}

func fillIntIfZero(cur int, src *int) int {
	if cur == 0 && src != nil {
		return *src
	}
	return cur
}

func externalToBook(ext SourceProposal) models.Book {
	var coverURL *string
	if ext.CoverURL != "" {
		coverURL = &ext.CoverURL
	}
	var isbn13 *string
	if ext.ISBN13 != "" {
		isbn13 = &ext.ISBN13
	}
	var description *string
	if ext.Description != "" {
		description = &ext.Description
	}
	var pageCount *int
	if ext.PageCount != 0 {
		pageCount = &ext.PageCount
	}

	// Hand-entered books (Source "manual"/"") keep a NULL metadata source.
	var metadataSource *string
	if ext.Source != "" && ext.Source != sourceManual {
		source := ext.Source
		metadataSource = &source
	}

	return models.Book{ //nolint:exhaustruct //optional fields
		Title:          ext.Title,
		Authors:        authorname.NormalizeAll(ext.Authors),
		ISBN13:         isbn13,
		CoverURL:       coverURL,
		Description:    description,
		PageCount:      pageCount,
		MetadataSource: metadataSource,
	}
}

// ListKoboSyncBooks returns the user's kobo-sync books that have a ready KEPUB.
func (s *BookService) ListKoboSyncBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboSyncBook, error) {
	return s.books.ListKoboSyncBooks(ctx, userID)
}

// UpdateKoboLastSyncedConverterVersion records the converter version last sent
// to the device, so the next sync can detect a regenerated file.
func (s *BookService) UpdateKoboLastSyncedConverterVersion(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	converterVersion int16,
) error {
	return s.books.UpdateKoboLastSyncedConverterVersion(
		ctx, userID, bookID, converterVersion)
}

// ListKoboRemovals returns books tombstoned for removal from the user's Kobo.
func (s *BookService) ListKoboRemovals(
	ctx context.Context,
	userID string,
) ([]models.KoboRemoval, error) {
	return s.books.ListKoboRemovals(ctx, userID)
}

// GetKoboSyncBook returns one kobo-sync book with a ready file, or
// ErrResourceNotFound.
func (s *BookService) GetKoboSyncBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (models.KoboSyncBook, error) {
	return s.books.GetKoboSyncBook(ctx, userID, bookID)
}

// UpdateReadingProgress upserts a resumable reading position (source
// web/kobo/manual; percent clamped to 0-100). A kobo update lowering the stored
// percent is dropped: devices re-report possibly stale local bookmarks (e.g.
// after a KEPUB re-download). web/manual reflect explicit user action.
func (s *BookService) UpdateReadingProgress(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	source string,
	percent int,
	location *string,
) error {
	if source != models.ReadingSourceWeb &&
		source != models.ReadingSourceKobo &&
		source != models.ReadingSourceManual {
		return fmt.Errorf("invalid reading source %q", source)
	}
	if percent < 0 {
		percent = 0
	}
	if percent > models.MaxProgressPercent {
		percent = models.MaxProgressPercent
	}

	if source == models.ReadingSourceKobo {
		// Any error here (including "no existing state") just means there's
		// nothing to regress against — fall through to the upsert below.
		if existing, err := s.readingState.Get(ctx, userID, bookID); err == nil &&
			percent < existing.Percent {
			return nil
		}
	}

	if err := s.readingState.Upsert(
		ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:   userID,
			BookID:   bookID,
			Source:   source,
			Percent:  percent,
			Location: location,
		},
	); err != nil {
		return err
	}

	// Non-zero progress promotes to-read/dropped to currently-reading.
	if percent > 0 {
		return s.books.UpdateLibraryProgress(ctx, userID, bookID, percent)
	}
	return nil
}

// GetReadingState returns the current resumable position for a book.
func (s *BookService) GetReadingState(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookReadingState, error) {
	return s.readingState.Get(ctx, userID, bookID)
}

// SetContentHTML stores a book's readability-extracted article body.
func (s *BookService) SetContentHTML(
	ctx context.Context,
	bookID uuid.UUID,
	html string,
) error {
	return s.books.SetBookContentHTML(ctx, bookID, html)
}

// SetAddedAt overwrites a user's library-add timestamp for a book.
func (s *BookService) SetAddedAt(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	addedAt time.Time,
) error {
	return s.books.UpdateUserBookAddedAt(ctx, userID, bookID, addedAt)
}

// GetContentHTML returns the stored article HTML, or "" if none.
func (s *BookService) GetContentHTML(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (string, error) {
	html, err := s.books.GetBookContentHTML(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	if html == nil {
		return "", nil
	}
	return *html, nil
}

// ListReadingStates returns all the user's reading states by book ID, avoiding
// N+1 GetReadingState calls.
func (s *BookService) ListReadingStates(
	ctx context.Context,
	userID string,
) (map[uuid.UUID]*models.BookReadingState, error) {
	rows, err := s.readingState.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	index := make(map[uuid.UUID]*models.BookReadingState, len(rows))
	for i := range rows {
		index[rows[i].BookID] = &rows[i]
	}
	return index, nil
}

// UpdateProgress persists progress: pages mode tracks current_page, percent
// mode tracks progress_percent (clamped 0-100).
func (s *BookService) UpdateProgress(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	mode string,
	currentPage int,
	progressPercent int,
) error {
	if mode != models.ProgressModePages && mode != models.ProgressModePercent {
		return fmt.Errorf("invalid progress mode %q", mode)
	}
	if currentPage < 0 {
		currentPage = 0
	}
	if progressPercent < 0 {
		progressPercent = 0
	}
	if progressPercent > models.MaxProgressPercent {
		progressPercent = models.MaxProgressPercent
	}

	return s.books.UpdateProgress(
		ctx, userID, bookID, mode, currentPage, progressPercent,
	)
}

// tombstoneIfKoboSynced tombstones a kobo-synced book. Must run before the
// delete, or the device copy can never be un-synced.
func (s *BookService) tombstoneIfKoboSynced(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil
		}
		return err
	}
	if !slices.Contains(ub.Tags, models.TagKoboSync) {
		return nil
	}
	return s.books.UpsertKoboRemoval(ctx, userID, bookID)
}

// RemoveFromLibrary removes a book and the caller's files and reading state;
// the catalog row and its R2 objects go too once no library references it.
// R2 deletes are best-effort; the daily storage scan sweeps leftovers.
//
//nolint:gocognit // linear cleanup sequence
func (s *BookService) RemoveFromLibrary(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	if err := s.tombstoneIfKoboSynced(ctx, userID, bookID); err != nil {
		return err
	}

	files, err := s.bookFiles.ListByBook(ctx, userID, bookID)
	if err != nil {
		return err
	}

	if _, err = s.bookFiles.DeleteByUserBook(ctx, userID, bookID); err != nil {
		return err
	}

	for _, f := range files {
		if f.StorageKey == "" {
			continue
		}
		remaining, countErr := s.bookFiles.CountByStorageKey(ctx, f.StorageKey)
		if countErr != nil {
			s.logger.Warn("failed to count references for book file",
				"key", f.StorageKey, "err", countErr)
			continue
		}
		if remaining > 0 {
			continue
		}
		delErr := objectstore.DeleteWithRetry(ctx, s.objectStore, f.StorageKey)
		if delErr != nil {
			s.logger.Error("failed to delete book file from object store",
				"key", f.StorageKey, "err", delErr)
		}
	}

	if err = s.readingState.DeleteByBook(ctx, userID, bookID); err != nil {
		return err
	}

	if err = s.books.DeleteUserBook(ctx, userID, bookID); err != nil {
		return err
	}

	deleted, err := s.books.DeleteOrphanedBook(ctx, bookID)
	if err != nil {
		return err
	}
	if deleted {
		for _, key := range []string{bookCoverKey(bookID), bookCoverMissingKey(bookID)} {
			if delErr := objectstore.DeleteWithRetry(ctx, s.objectStore, key); delErr != nil {
				s.logger.Error("failed to delete book cover from object store",
					"key", key, "err", delErr)
			}
		}
	}

	return nil
}

// ListCatalogBooks returns all catalog books ordered by title.
func (s *BookService) ListCatalogBooks(
	ctx context.Context,
) ([]models.Book, error) {
	return s.books.ListCatalogBooks(ctx)
}

// FindDuplicates groups likely-duplicate books across the whole catalog, with
// the caller's user_book data overlaid.
func (s *BookService) FindDuplicates(
	ctx context.Context,
	userID string,
) ([]DuplicateGroup, error) {
	lib, err := s.books.GetCatalogWithUserOverlay(ctx, userID)
	if err != nil {
		return nil, err
	}
	return FindDuplicateGroups(lib), nil
}

// consolidateUserBookData merges loserBookIDs into the winner for one user,
// creating a winner row if needed and skipping unowned losers. Returns the
// storage keys of deleted duplicate files for global R2 cleanup.
//
//nolint:cyclop,funlen,gocognit,gocyclo // per-user merge; cannot split further
func (s *BookService) consolidateUserBookData(
	ctx context.Context,
	userID string,
	winnerBookID uuid.UUID,
	loserBookIDs []uuid.UUID,
	statusOverride *string,
) ([]string, error) {
	winner, err := s.books.GetUserBook(ctx, userID, winnerBookID)
	winnerOwned := true
	if err != nil {
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, fmt.Errorf("load winner for user %s: %w", userID, err)
		}
		winnerOwned = false
		winner = &models.UserBook{ //nolint:exhaustruct // zero-value seed for unowned winner
			UserID:         userID,
			BookID:         winnerBookID,
			Tags:           []string{},
			FinishedAt:     []time.Time{},
			ShelfPositions: make(map[string]int),
		}
	}

	var ownedLosers []uuid.UUID
	for _, loserID := range loserBookIDs {
		loser, loserErr := s.books.GetUserBook(ctx, userID, loserID)
		if errors.Is(loserErr, database.ErrResourceNotFound) {
			continue // user doesn't own this loser — skip
		}
		if loserErr != nil {
			return nil, fmt.Errorf(
				"load loser %s for user %s: %w", loserID, userID, loserErr,
			)
		}
		ownedLosers = append(ownedLosers, loserID)

		for _, tag := range loser.Tags {
			if !slices.Contains(winner.Tags, tag) {
				winner.Tags = append(winner.Tags, tag)
			}
		}
		for _, ft := range loser.FinishedAt {
			found := false
			for _, wft := range winner.FinishedAt {
				if wft.Equal(ft) {
					found = true
					break
				}
			}
			if !found {
				winner.FinishedAt = append(winner.FinishedAt, ft)
			}
		}
		for shelf, pos := range loser.ShelfPositions {
			if _, ok := winner.ShelfPositions[shelf]; !ok {
				winner.ShelfPositions[shelf] = pos
			}
		}
		if statusRank(loser.Status) > statusRank(winner.Status) {
			winner.Status = loser.Status
		}
		if winner.Rating == nil && loser.Rating != nil {
			winner.Rating = loser.Rating
		}
		if loser.CurrentPage > winner.CurrentPage {
			winner.CurrentPage = loser.CurrentPage
			winner.ProgressMode = loser.ProgressMode
		}
		if loser.ProgressPercent > winner.ProgressPercent {
			winner.ProgressPercent = loser.ProgressPercent
		}
	}

	if !winnerOwned && len(ownedLosers) == 0 {
		return nil, nil
	}

	if statusOverride != nil && *statusOverride != "" {
		winner.Status = *statusOverride
	}

	if err = s.books.UpsertUserBook(ctx, *winner); err != nil {
		return nil, fmt.Errorf("upsert winner for user %s: %w", userID, err)
	}

	var deletedKeys []string
	for _, loserID := range ownedLosers {
		keys, repointErr := s.bookFiles.RepointAndDedup(
			ctx, userID, loserID, winnerBookID,
		)
		if repointErr != nil {
			return deletedKeys, fmt.Errorf(
				"repoint files loser %s user %s: %w", loserID, userID, repointErr,
			)
		}
		deletedKeys = append(deletedKeys, keys...)
	}

	winnerState, err := s.readingState.Get(ctx, userID, winnerBookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return deletedKeys, fmt.Errorf(
			"get winner reading state user %s: %w", userID, err,
		)
	}
	for _, loserID := range ownedLosers {
		loserState, loserErr := s.readingState.Get(ctx, userID, loserID)
		if errors.Is(loserErr, database.ErrResourceNotFound) {
			continue
		}
		if loserErr != nil {
			return deletedKeys, fmt.Errorf(
				"get reading state loser %s user %s: %w", loserID, userID, loserErr,
			)
		}
		if winnerState == nil {
			copied := *loserState
			copied.BookID = winnerBookID
			if upsertErr := s.readingState.Upsert(ctx, copied); upsertErr != nil {
				return deletedKeys, fmt.Errorf(
					"upsert reading state loser %s user %s: %w",
					loserID, userID, upsertErr,
				)
			}
			winnerState = &copied
		}
		if delErr := s.readingState.DeleteByBook(ctx, userID, loserID); delErr != nil {
			return deletedKeys, fmt.Errorf(
				"delete reading state loser %s user %s: %w", loserID, userID, delErr,
			)
		}
	}

	for _, loserID := range ownedLosers {
		if delErr := s.books.DeleteUserBook(ctx, userID, loserID); delErr != nil {
			return deletedKeys, fmt.Errorf(
				"delete user_book loser %s user %s: %w", loserID, userID, delErr,
			)
		}
	}

	return deletedKeys, nil
}

// MergeBooks consolidates loserBookIDs into winnerBookID for every owning user,
// then deletes the loser catalog rows and applies the resolved overrides.
// resolvedStatus applies only to the caller's entry. R2 objects are deleted
// only when no other row references them.
//
//nolint:gocognit // global multi-entity merge; cannot split further
func (s *BookService) MergeBooks(
	ctx context.Context,
	callerID string,
	winnerBookID uuid.UUID,
	loserBookIDs []uuid.UUID,
	resolvedMetadata *models.Book,
	resolvedCoverSourceBookID *uuid.UUID,
	resolvedStatus *string,
) (uint32, []string, error) {
	if len(loserBookIDs) == 0 {
		return 0, nil, nil
	}

	allIDs := append([]uuid.UUID{winnerBookID}, loserBookIDs...)
	affectedUsers, err := s.books.ListUserBookOwners(ctx, allIDs)
	if err != nil {
		return 0, nil, fmt.Errorf("list affected users: %w", err)
	}
	callerIncluded := false
	for _, uid := range affectedUsers {
		if uid == callerID {
			callerIncluded = true
			break
		}
	}
	if !callerIncluded {
		affectedUsers = append(affectedUsers, callerID)
	}

	var allDeletedKeys []string
	for _, uid := range affectedUsers {
		var override *string
		if uid == callerID {
			override = resolvedStatus
		}
		keys, consolidateErr := s.consolidateUserBookData(
			ctx, uid, winnerBookID, loserBookIDs, override,
		)
		if consolidateErr != nil {
			return 0, affectedUsers, consolidateErr
		}
		allDeletedKeys = append(allDeletedKeys, keys...)
	}

	var totalDeletedFiles uint32
	seen := make(map[string]bool, len(allDeletedKeys))
	for _, key := range allDeletedKeys {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		totalDeletedFiles++
		remaining, countErr := s.bookFiles.CountByStorageKey(ctx, key)
		if countErr != nil {
			s.logger.Warn("failed to count references for book file",
				"key", key, "err", countErr)
			continue
		}
		if remaining > 0 {
			continue
		}
		if delErr := objectstore.DeleteWithRetry(ctx, s.objectStore, key); delErr != nil {
			s.logger.Error("failed to delete book file from object store",
				"key", key, "err", delErr)
		}
	}

	for _, loserID := range loserBookIDs {
		if _, delErr := s.books.DeleteOrphanedBook(ctx, loserID); delErr != nil {
			return totalDeletedFiles, affectedUsers, fmt.Errorf(
				"delete orphaned book for loser %s: %w", loserID, delErr,
			)
		}
	}

	if resolvedMetadata != nil {
		resolvedMetadata.ID = winnerBookID
		if updateErr := s.books.UpdateBookByID(ctx, *resolvedMetadata); updateErr != nil {
			return totalDeletedFiles, affectedUsers, fmt.Errorf(
				"apply resolved metadata: %w", updateErr,
			)
		}
	}
	if resolvedCoverSourceBookID != nil &&
		*resolvedCoverSourceBookID != winnerBookID {
		if coverErr := s.applyCoverSource(
			ctx, winnerBookID, *resolvedCoverSourceBookID,
		); coverErr != nil {
			return totalDeletedFiles, affectedUsers, fmt.Errorf(
				"apply resolved cover: %w", coverErr,
			)
		}
	}

	return totalDeletedFiles, affectedUsers, nil
}

// applyCoverSource copies the source book's cover onto the winner and
// refreshes its R2 cover cache.
func (s *BookService) applyCoverSource(
	ctx context.Context,
	winnerBookID uuid.UUID,
	sourceBookID uuid.UUID,
) error {
	source, err := s.books.GetBookByID(ctx, sourceBookID)
	if err != nil {
		return fmt.Errorf("load cover source book: %w", err)
	}

	winner, err := s.books.GetBookByID(ctx, winnerBookID)
	if err != nil {
		return fmt.Errorf("load winner book for cover update: %w", err)
	}

	winner.CoverURL = source.CoverURL
	if updateErr := s.books.UpdateBookByID(ctx, *winner); updateErr != nil {
		return fmt.Errorf("write cover_url to winner: %w", updateErr)
	}

	if winner.CoverURL != nil && *winner.CoverURL != "" {
		cacheErr := s.cacheCoverFromURL(ctx, winnerBookID, *winner.CoverURL)
		if cacheErr != nil {
			s.logger.Warn("failed to cache merged book cover",
				"bookID", winnerBookID, "err", cacheErr)
		}
		return nil
	}

	if clearErr := s.clearCoverCache(ctx, winnerBookID); clearErr != nil {
		s.logger.Warn("failed to clear book cover cache",
			"bookID", winnerBookID, "err", clearErr)
	}
	return nil
}
