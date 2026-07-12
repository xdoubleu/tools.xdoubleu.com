package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/books"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

// providerOpenLibrary identifies Open Library as a metadata source/provider.
const providerOpenLibrary = "openlibrary"

type BookService struct {
	logger       *slog.Logger
	books        *repositories.BooksRepository
	bookFiles    *repositories.BookFilesRepository
	objectStore  objectstore.Client
	readingState *repositories.BookReadingStateRepository
	external     openlibrary.Client
	googleBooks  googlebooks.Client
	uniCat       unicat.Client
	// booksResync overrides s.books for the resync path in unit tests.
	// Nil in production — resyncRepo() falls back to s.books.
	booksResync booksResyncSource
}

// SearchLibrary searches the user's own library by title/author substring.
func (s *BookService) SearchLibrary(
	ctx context.Context,
	userID string,
	query string,
) ([]models.UserBook, error) {
	return s.books.SearchLibrary(ctx, userID, query)
}

// SearchExternal searches the Open Library API for books matching the query.
func (s *BookService) SearchExternal(
	ctx context.Context,
	query string,
) ([]openlibrary.ExternalBook, error) {
	return s.external.Search(ctx, query)
}

// GetExternal fetches a single book from an external provider by its
// provider-scoped ID, for the not-in-library detail page. Only "openlibrary"
// is supported today; other values return ErrNotFound.
func (s *BookService) GetExternal(
	ctx context.Context,
	provider string,
	providerID string,
) (*openlibrary.ExternalBook, error) {
	if provider != providerOpenLibrary {
		return nil, openlibrary.ErrNotFound
	}
	return s.external.Get(ctx, providerID)
}

// SetBookISBN sets the isbn13 of the given catalog book.
// Returns database.ErrResourceNotFound when the book doesn't exist, or
// database.ErrResourceConflict when another catalog row already holds the ISBN.
func (s *BookService) SetBookISBN(
	ctx context.Context,
	bookID uuid.UUID,
	isbn13 string,
) error {
	// Normalize before the pre-check and write so that hyphenated input
	// ("978-94-6310-738-9") matches the same unique index entry as the
	// plain form ("9789463107389") that providers store.
	isbn13 = normalizeISBN(isbn13)

	book, err := s.books.GetBookByID(ctx, bookID)
	if err != nil {
		return err
	}

	// Pre-check: reject if the ISBN is already assigned to a different row.
	existing, err := s.books.GetCatalogBookByISBN13(ctx, isbn13)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}
	if existing != nil && existing.ID != book.ID {
		return database.ErrResourceConflict
	}

	book.ISBN13 = &isbn13
	return s.books.UpdateBookByID(ctx, *book)
}

func (s *BookService) AddToLibrary(
	ctx context.Context,
	userID string,
	ext openlibrary.ExternalBook,
	status string,
	initialTags []string,
) (*models.UserBook, error) {
	book := externalToBook(s.enrichByISBN(ctx, ext))
	saved, err := s.books.UpsertBook(ctx, book)
	if err != nil {
		return nil, err
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

// registerCustomShelf records a custom (non-built-in) status in the shelves
// registry so it persists even after its last book is moved off it. Built-in
// statuses are never stored — they're always implicit.
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
	// Disabling kobo-sync leaves any already-downloaded copy on the device;
	// tombstone it so the next sync actively removes it. Re-enabling clears
	// a stale tombstone from a prior disable.
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

// builtInStatuses are the fixed reading-state values that map to the three
// top-level library buckets. They cannot be renamed or deleted via the
// shelf-management RPCs because they carry semantic meaning (progress gating,
// rating unlock, etc.).
//
//nolint:gochecknoglobals // effectively a constant set
var builtInStatuses = map[string]bool{
	models.StatusToRead:  true,
	models.StatusReading: true,
	models.StatusRead:    true,
	models.StatusDropped: true,
}

// ListShelves returns every custom shelf name registered for the user,
// including shelves with zero books currently on them.
func (s *BookService) ListShelves(
	ctx context.Context,
	userID string,
) ([]string, error) {
	return s.books.ListShelves(ctx, userID)
}

// CreateShelf registers a new custom shelf with no books on it yet. Returns
// an error if the name is empty or a built-in status.
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
// Returns an error if old or new name is a built-in status.
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

// DeleteShelf moves all books on a custom shelf (= status) to targetName,
// effectively deleting the shelf. Returns an error if name or targetName is
// a built-in status (built-in target is allowed — e.g. move to "to-read").
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

// ImportFromCSV parses a Goodreads CSV export and upserts all entries into the library.
// Returns the number of entries successfully imported.
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

// enrichByISBN best-effort fills missing description/page-count/cover on a book
// by looking it up in Open Library by ISBN13. Open Library's search results omit
// the description and sometimes the page count, so this is run when a book is
// added to the library. Lookup failures are logged and the original book is
// returned unchanged — enrichment never blocks an add.
func (s *BookService) enrichByISBN(
	ctx context.Context,
	ext openlibrary.ExternalBook,
) openlibrary.ExternalBook {
	if ext.ISBN13 == nil || *ext.ISBN13 == "" {
		return ext
	}
	if ext.Description != nil && ext.PageCount != nil && ext.CoverURL != nil {
		return ext
	}

	detail, err := s.external.GetByISBN(ctx, *ext.ISBN13)
	if err != nil || detail == nil {
		if err != nil && !errors.Is(err, openlibrary.ErrNotFound) {
			s.logger.WarnContext(ctx, "open library ISBN lookup failed", "error", err)
		}
		return ext
	}

	if ext.Description == nil {
		ext.Description = detail.Description
	}
	if ext.PageCount == nil {
		ext.PageCount = detail.PageCount
	}
	if ext.CoverURL == nil {
		ext.CoverURL = detail.CoverURL
	}
	return ext
}

func externalToBook(ext openlibrary.ExternalBook) models.Book {
	coverURL := ext.CoverURL
	if coverURL == nil {
		if fallback := openlibrary.CoverURLByISBN(ext.ISBN13); fallback != "" {
			coverURL = &fallback
		}
	}

	// Record provenance only for books whose metadata actually came from a
	// source — hand-entered books (Provider "manual"/"") stay NULL.
	var metadataSource *string
	if ext.Provider == providerOpenLibrary {
		source := ext.Provider
		metadataSource = &source
	}

	return models.Book{ //nolint:exhaustruct //optional fields
		Title:          ext.Title,
		Authors:        ext.Authors,
		ISBN13:         ext.ISBN13,
		CoverURL:       coverURL,
		Description:    ext.Description,
		PageCount:      ext.PageCount,
		MetadataSource: metadataSource,
	}
}

// ListKoboSyncBooks returns every book the user has enabled Kobo sync for and
// that has a ready KEPUB — the exact set served by the sync protocol routes.
func (s *BookService) ListKoboSyncBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboSyncBook, error) {
	return s.books.ListKoboSyncBooks(ctx, userID)
}

// ListKoboRemovals returns books tombstoned for active removal from the
// user's Kobo device.
func (s *BookService) ListKoboRemovals(
	ctx context.Context,
	userID string,
) ([]models.KoboRemoval, error) {
	return s.books.ListKoboRemovals(ctx, userID)
}

// GetKoboSyncBook returns a single kobo-sync book by ID for the user.
// Returns database.ErrResourceNotFound when the book is not in the user's
// kobo-sync list or has no ready file.
func (s *BookService) GetKoboSyncBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (models.KoboSyncBook, error) {
	return s.books.GetKoboSyncBook(ctx, userID, bookID)
}

// UpdateReadingProgress upserts a resumable reading position for a book.
// source must be one of web/kobo/manual; percent is clamped to 0-100.
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

	// Reflect progress on the library entry and promote from to-read / dropped
	// → currently-reading whenever progress is non-zero. No-op for books
	// already reading, read, or not in the library at all.
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

// ListReadingStates returns all reading states for the user, indexed by
// book ID. Use this instead of per-book GetReadingState when processing a
// batch of books to avoid N+1 queries.
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

// UpdateProgress validates and persists reading-progress for a user_book. The
// mode selects which value is authoritative: pages mode tracks current_page,
// percent mode tracks progress_percent (clamped to 0-100).
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

// tombstoneIfKoboSynced records a removal tombstone when bookID currently has
// the kobo-sync tag. Deleting a kobo-synced book leaves a copy on the device
// with nothing left server-side to un-sync it later, so this must run before
// the book is actually deleted.
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

// deleteOrphanedFiles best-effort deletes each file's R2 object once no other
// book_files row still references the same storage key. Failures are logged
// and skipped — the daily storage scan sweeps any leftovers.
func (s *BookService) deleteOrphanedFiles(
	ctx context.Context,
	files []models.BookFile,
) {
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
		if delErr := s.objectStore.Delete(ctx, f.StorageKey); delErr != nil {
			s.logger.Warn("failed to delete book file from object store",
				"key", f.StorageKey, "err", delErr)
		}
	}
}

// RemoveFromLibrary removes a single book from the caller's own library:
// their uploaded files (DB rows + R2 objects, refcount-safe), reading state,
// and user_books entry. If the book is no longer referenced by any user's
// library afterwards, the shared catalog row and its R2 objects (files and
// cover) are deleted too. R2 deletes are best-effort — a failed object delete
// is logged and skipped; the daily storage scan sweeps any leftovers.
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

	s.deleteOrphanedFiles(ctx, files)

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
			if delErr := s.objectStore.Delete(ctx, key); delErr != nil {
				s.logger.Warn("failed to delete book cover from object store",
					"key", key, "err", delErr)
			}
		}
	}

	return nil
}

// ListCatalogBooks returns all catalog books ordered by title. Used by the
// admin selective-resync tool.
func (s *BookService) ListCatalogBooks(
	ctx context.Context,
) ([]models.Book, error) {
	return s.books.ListCatalogBooks(ctx)
}

// FindDuplicates returns groups of catalog entries judged to be duplicates of
// the same book. It scans the entire catalog (not just the caller's library)
// with the caller's user_book data overlaid, so catalog-level duplicates are
// visible even when the user has only one (or none) of them in their library.
// Callers that need to act on a match can pass the returned BookIDs to
// MergeBooks regardless of whether the entry is in their own library.
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

// consolidateUserBookData merges all loserBookIDs into the winner for a single
// user. It is ownership-tolerant: if the user doesn't own the winner a new
// user_books row is created for them; if they don't own a particular loser that
// loser is silently skipped for this user.
//
// Returns the storage_keys of any duplicate book_files that were deleted so the
// caller can do a global refcount-safe R2 cleanup after all users are processed.
//
//nolint:cyclop,funlen,gocognit,gocyclo // per-user merge; cannot split further
func (s *BookService) consolidateUserBookData(
	ctx context.Context,
	userID string,
	winnerBookID uuid.UUID,
	loserBookIDs []uuid.UUID,
	statusOverride *string,
) ([]string, error) {
	// Load winner row; seed a zero entry if this user doesn't own it yet.
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

	// Load each loser; union data into winner, collect owned loser IDs.
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

	// Nothing to do if this user has no ownership stake at all.
	if !winnerOwned && len(ownedLosers) == 0 {
		return nil, nil
	}

	if statusOverride != nil && *statusOverride != "" {
		winner.Status = *statusOverride
	}

	if err = s.books.UpsertUserBook(ctx, *winner); err != nil {
		return nil, fmt.Errorf("upsert winner for user %s: %w", userID, err)
	}

	// Repoint / dedup book_files from each owned loser.
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

	// Consolidate reading state.
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

	// Delete loser user_books rows.
	for _, loserID := range ownedLosers {
		if delErr := s.books.DeleteUserBook(ctx, userID, loserID); delErr != nil {
			return deletedKeys, fmt.Errorf(
				"delete user_book loser %s user %s: %w", loserID, userID, delErr,
			)
		}
	}

	return deletedKeys, nil
}

// MergeBooks is a global admin merge: it consolidates loserBookIDs into
// winnerBookID for every user who owns any of the involved books, then deletes
// the now-orphaned loser catalog rows.
//
// For each affected user:
//  1. Union tags, finished_at, shelf_positions; prefer the highest-ranked
//     status; keep winner's rating / progress, fall back to loser's if unset.
//  2. Repoint book_files from each owned loser to the winner.
//  3. Consolidate reading state.
//  4. Delete loser user_books rows.
//
// After all users: delete orphaned loser catalog rows, apply resolvedMetadata
// and resolvedCoverSourceBookID. resolvedStatus applies only to the caller's
// winner entry.
//
// Returns (deletedFiles, affectedUserIDs, error). R2 objects are only deleted
// when no other row still references them.
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

	// Collect all users who own any of the involved catalog books.
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

	// Per-user consolidation.
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

	// Refcount-safe R2 cleanup (global — after all users' files are repointed).
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
		if delErr := s.objectStore.Delete(ctx, key); delErr != nil {
			s.logger.Warn("failed to delete book file from object store",
				"key", key, "err", delErr)
		}
	}

	// Delete now-orphaned loser catalog rows.
	for _, loserID := range loserBookIDs {
		if _, delErr := s.books.DeleteOrphanedBook(ctx, loserID); delErr != nil {
			return totalDeletedFiles, affectedUsers, fmt.Errorf(
				"delete orphaned book for loser %s: %w", loserID, delErr,
			)
		}
	}

	// Apply catalog-level overrides.
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

// applyCoverSource copies the source book's cover_url onto the winner catalog
// row and clears the winner's R2 cover cache so the next request re-fetches
// the image from the new URL.
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

	// Clear cached cover images so the next proxy request re-fetches.
	for _, key := range []string{
		bookCoverKey(winnerBookID),
		bookCoverMissingKey(winnerBookID),
	} {
		if exists, checkErr := s.objectStore.Exists(ctx, key); checkErr != nil {
			s.logger.Warn(
				"failed to check cover cache key",
				"key",
				key,
				"err",
				checkErr,
			)
		} else if exists {
			if delErr := s.objectStore.Delete(ctx, key); delErr != nil {
				s.logger.Warn(
					"failed to clear cover cache key",
					"key", key, "err", delErr,
				)
			}
		}
	}

	return nil
}
