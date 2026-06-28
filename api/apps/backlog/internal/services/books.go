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

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/books"
	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
)

type BookService struct {
	logger       *slog.Logger
	books        *repositories.BooksRepository
	bookFiles    *repositories.BookFilesRepository
	objectStore  objectstore.Client
	readingState *repositories.BookReadingStateRepository
	external     openlibrary.Client
	googleBooks  googlebooks.Client
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

// SetBookISBN sets the isbn13 of the given catalog book.
// Returns database.ErrResourceNotFound when the book doesn't exist, or
// database.ErrResourceConflict when another catalog row already holds the ISBN.
func (s *BookService) SetBookISBN(
	ctx context.Context,
	bookID uuid.UUID,
	isbn13 string,
) error {
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

	return s.books.GetUserBook(ctx, userID, saved.ID)
}

func (s *BookService) UpdateStatus(
	ctx context.Context,
	_ string,
	ub models.UserBook,
) error {
	return s.books.UpsertUserBook(ctx, ub)
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

	return s.books.UpdateTags(
		ctx, userID, bookID, newTags,
		slices.Contains(newTags, models.TagKoboSync),
	)
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

	return models.Book{ //nolint:exhaustruct //optional fields
		Title:       ext.Title,
		Authors:     ext.Authors,
		ISBN13:      ext.ISBN13,
		CoverURL:    coverURL,
		Description: ext.Description,
		PageCount:   ext.PageCount,
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

	// Promote from to-read / dropped → currently-reading whenever progress
	// is non-zero. No-op for books already reading, read, or not in the
	// library at all.
	if percent > 0 {
		return s.books.PromoteToReading(ctx, userID, bookID)
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

// ClearLibrary removes all per-user books data: uploaded files (DB rows + R2
// objects), reading state, and user_books entries. The shared backlog.books
// catalog is never touched. R2 deletes are best-effort — a failed object delete
// is logged and skipped so the user can retry without being blocked.
//
// R2 objects shared with other users (content-addressed canonical blobs) are
// only deleted when no other book_files row still references them; this is
// checked after DeleteByUser so the count already excludes this user's rows.
func (s *BookService) ClearLibrary(
	ctx context.Context,
	userID string,
) (uint32, uint32, error) {
	keys, err := s.bookFiles.StorageKeysByUser(ctx, userID)
	if err != nil {
		return 0, 0, err
	}

	fileCount, err := s.bookFiles.DeleteByUser(ctx, userID)
	if err != nil {
		return 0, 0, err
	}

	// Deduplicate keys so we issue at most one refcount check per object.
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, already := seen[key]; already {
			continue
		}
		seen[key] = struct{}{}

		// Only delete the R2 object when no other row still references it.
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

	if err = s.readingState.DeleteByUser(ctx, userID); err != nil {
		return 0, 0, err
	}

	bookCount, err := s.books.DeleteUserBooks(ctx, userID)
	if err != nil {
		return 0, 0, err
	}

	//nolint:gosec // row counts are safe to downcast
	return uint32(bookCount), uint32(fileCount), nil
}

// ListCatalogBooks returns all catalog books ordered by title. Used by the
// admin selective-resync tool.
func (s *BookService) ListCatalogBooks(
	ctx context.Context,
) ([]models.Book, error) {
	return s.books.ListCatalogBooks(ctx)
}

// FindDuplicates returns groups of library entries judged to be duplicates of
// the same book. It loads the user's full library and delegates to the pure
// FindDuplicateGroups helper.
func (s *BookService) FindDuplicates(
	ctx context.Context,
	userID string,
) ([]DuplicateGroup, error) {
	lib, err := s.books.GetLibrary(ctx, userID)
	if err != nil {
		return nil, err
	}
	return FindDuplicateGroups(lib), nil
}

// MergeBooks consolidates loserBookIDs into winnerBookID:
//  1. Union tags, finished_at, shelf_positions; prefer the highest-ranked
//     status (custom shelf > read > currently-reading > to-read > dropped);
//     keep winner's rating, fall back to a loser's if unset.
//  2. Repoint book_files from each loser to the winner (dedupe by format+checksum).
//  3. Consolidate book_reading_state: if the winner has no state, copy the best
//     loser state onto it, then delete all loser states.
//  4. Delete the loser user_books rows.
//
// resolvedStatus, when non-nil and non-empty, overrides the auto-consolidated
// status after the consolidation loop runs.
//
// R2 objects are only deleted when no other row still references them (same
// discipline as ClearLibrary). Errors at individual merge steps are returned
// immediately; partial progress can be cleaned up by retrying.
//
//nolint:cyclop,funlen,gocognit,gocyclo // multi-entity merge; cannot split further
func (s *BookService) MergeBooks(
	ctx context.Context,
	userID string,
	winnerBookID uuid.UUID,
	loserBookIDs []uuid.UUID,
	resolvedMetadata *models.Book,
	resolvedCoverSourceBookID *uuid.UUID,
	resolvedStatus *string,
) (uint32, error) {
	if len(loserBookIDs) == 0 {
		return 0, nil
	}

	// --- 1. Load winner ---
	winner, err := s.books.GetUserBook(ctx, userID, winnerBookID)
	if err != nil {
		return 0, fmt.Errorf("load winner: %w", err)
	}

	// --- 2. Load losers and consolidate winner fields ---
	for _, loserID := range loserBookIDs {
		loser, loserErr := s.books.GetUserBook(ctx, userID, loserID)
		if loserErr != nil {
			return 0, fmt.Errorf("load loser %s: %w", loserID, loserErr)
		}

		// Union tags (deduplicate).
		for _, t := range loser.Tags {
			if !slices.Contains(winner.Tags, t) {
				winner.Tags = append(winner.Tags, t)
			}
		}

		// Union finished_at timestamps (deduplicate by truncating to day).
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

		// Merge shelf_positions: winner's positions take precedence.
		for shelf, pos := range loser.ShelfPositions {
			if _, ok := winner.ShelfPositions[shelf]; !ok {
				winner.ShelfPositions[shelf] = pos
			}
		}

		// Pick the more-progressed status.
		if statusRank(loser.Status) > statusRank(winner.Status) {
			winner.Status = loser.Status
		}

		// Keep winner's rating, fall back to loser if winner has none.
		if winner.Rating == nil && loser.Rating != nil {
			winner.Rating = loser.Rating
		}

		// Keep the higher progress values.
		if loser.CurrentPage > winner.CurrentPage {
			winner.CurrentPage = loser.CurrentPage
			winner.ProgressMode = loser.ProgressMode
		}
		if loser.ProgressPercent > winner.ProgressPercent {
			winner.ProgressPercent = loser.ProgressPercent
		}
	}

	// Apply the caller's explicit status override (after auto-consolidation).
	if resolvedStatus != nil && *resolvedStatus != "" {
		winner.Status = *resolvedStatus
	}

	// Persist consolidated winner.
	if err = s.books.UpsertUserBook(ctx, *winner); err != nil {
		return 0, fmt.Errorf("update winner: %w", err)
	}

	// --- 3. Repoint / dedup book_files and clean up orphaned blobs ---
	var totalDeletedFiles uint32
	for _, loserID := range loserBookIDs {
		keys, repointErr := s.bookFiles.RepointAndDedup(
			ctx, userID, loserID, winnerBookID,
		)
		if repointErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"repoint files for loser %s: %w", loserID, repointErr,
			)
		}

		for _, key := range keys {
			if key == "" {
				continue
			}

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
	}

	// --- 4. Consolidate reading state ---
	winnerState, err := s.readingState.Get(ctx, userID, winnerBookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return totalDeletedFiles, fmt.Errorf("get winner reading state: %w", err)
	}

	for _, loserID := range loserBookIDs {
		loserState, loserErr := s.readingState.Get(ctx, userID, loserID)
		if errors.Is(loserErr, database.ErrResourceNotFound) {
			continue
		}
		if loserErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"get reading state for loser %s: %w", loserID, loserErr,
			)
		}

		// Only copy if winner currently has no state.
		if winnerState == nil {
			copied := *loserState
			copied.BookID = winnerBookID
			if upsertErr := s.readingState.Upsert(ctx, copied); upsertErr != nil {
				return totalDeletedFiles, fmt.Errorf(
					"upsert reading state from loser %s: %w", loserID, upsertErr,
				)
			}
			winnerState = &copied
		}

		if delErr := s.readingState.DeleteByBook(ctx, userID, loserID); delErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"delete reading state for loser %s: %w", loserID, delErr,
			)
		}
	}

	// --- 5. Delete loser user_books rows ---
	for _, loserID := range loserBookIDs {
		if delErr := s.books.DeleteUserBook(ctx, userID, loserID); delErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"delete user_book for loser %s: %w", loserID, delErr,
			)
		}
	}

	// --- 6. Clean up orphaned loser catalog rows, then apply resolved metadata ---
	for _, loserID := range loserBookIDs {
		if delErr := s.books.DeleteOrphanedBook(ctx, loserID); delErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"delete orphaned book for loser %s: %w", loserID, delErr,
			)
		}
	}

	if resolvedMetadata != nil {
		resolvedMetadata.ID = winnerBookID
		if updateErr := s.books.UpdateBookByID(ctx, *resolvedMetadata); updateErr != nil {
			return totalDeletedFiles, fmt.Errorf(
				"apply resolved metadata: %w",
				updateErr,
			)
		}
	}

	if resolvedCoverSourceBookID != nil &&
		*resolvedCoverSourceBookID != winnerBookID {
		if coverErr := s.applyCoverSource(
			ctx, winnerBookID, *resolvedCoverSourceBookID,
		); coverErr != nil {
			return totalDeletedFiles, fmt.Errorf("apply resolved cover: %w", coverErr)
		}
	}

	return totalDeletedFiles, nil
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
