package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
)

// BookStore is the book, library and Kobo-state storage BookService uses.
type BookStore interface {
	BatchUpsert(
		ctx context.Context,
		userID string,
		books []models.Book,
		userBooks []models.UserBook,
	) error
	DeleteKoboRemoval(ctx context.Context, userID string, bookID uuid.UUID) error
	DeleteOrphanedBook(ctx context.Context, bookID uuid.UUID) (bool, error)
	DeleteShelf(
		ctx context.Context,
		userID string,
		oldName string,
		targetName string,
	) (uint32, error)
	DeleteTag(ctx context.Context, userID string, name string) (uint32, error)
	DeleteUserBook(ctx context.Context, userID string, bookID uuid.UUID) error
	EnsureShelf(ctx context.Context, userID string, name string) error
	FindUserBookByISBN13(
		ctx context.Context,
		userID string,
		isbn13 string,
	) (*models.UserBook, error)
	FindUserBookByTitleAndAuthor(
		ctx context.Context,
		userID string,
		title string,
		author string,
	) (*models.UserBook, error)
	GetBookByID(ctx context.Context, bookID uuid.UUID) (*models.Book, error)
	GetBookContentHTML(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) (*string, error)
	GetBooksByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Book, error)
	GetByStatus(
		ctx context.Context,
		userID string,
		status string,
	) ([]models.UserBook, error)
	GetCatalogBookByISBN13(ctx context.Context, isbn13 string) (*models.Book, error)
	GetCatalogWithUserOverlay(
		ctx context.Context,
		userID string,
	) ([]models.UserBook, error)
	GetFinishedDates(ctx context.Context, userID string) ([]time.Time, error)
	GetKoboStoreBook(
		ctx context.Context,
		userID string,
		entitlementID string,
	) (*models.KoboStoreBook, error)
	GetKoboSyncBook(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) (models.KoboSyncBook, error)
	GetLibrary(ctx context.Context, userID string) ([]models.UserBook, error)
	GetSourceStats(ctx context.Context) (*repositories.SourceStats, error)
	GetUserBook(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) (*models.UserBook, error)
	ListBooksInExactSources(
		ctx context.Context,
		sources []string,
	) ([]models.Book, error)
	ListCatalogBooks(ctx context.Context) ([]models.Book, error)
	ListKoboRemovals(ctx context.Context, userID string) ([]models.KoboRemoval, error)
	ListKoboStoreBooks(
		ctx context.Context,
		userID string,
	) ([]models.KoboStoreBookStatus, error)
	ListKoboSyncBooks(ctx context.Context, userID string) ([]models.KoboSyncBook, error)
	ListResyncProposals(ctx context.Context) ([]repositories.ResyncProposalRow, error)
	ListShelves(ctx context.Context, userID string) ([]string, error)
	ListUserBookOwners(ctx context.Context, bookIDs []uuid.UUID) ([]string, error)
	ListUserBooksInSeries(
		ctx context.Context,
		userID string,
		name string,
	) ([]models.UserBook, error)
	RecordKoboStoreOutcome(
		ctx context.Context,
		userID string,
		reading models.KoboStoreReading,
		outcome models.KoboStoreOutcome,
	) error
	RenameShelf(
		ctx context.Context,
		userID string,
		oldName string,
		newName string,
	) (uint32, error)
	RenameTag(
		ctx context.Context,
		userID string,
		oldName string,
		newName string,
	) (uint32, error)
	SearchLibrary(
		ctx context.Context,
		userID string,
		query string,
		limit int32,
		offset int32,
	) ([]models.UserBook, bool, error)
	SetBookContentHTML(ctx context.Context, bookID uuid.UUID, html string) error
	SetBookSeries(
		ctx context.Context,
		bookID uuid.UUID,
		series *models.BookSeries,
	) error
	SetSeriesTotal(ctx context.Context, name string, total int) error
	UpdateBookByID(ctx context.Context, book models.Book) error
	UpdateFinishedAt(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		finishedAt []time.Time,
	) error
	UpdateKoboLastSynced(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		converterVersion int16,
		coverImageID string,
	) error
	UpdateLibraryProgress(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		percent int,
	) error
	UpdateProgress(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		mode string,
		currentPage int,
		progressPercent int,
	) error
	UpdateTags(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		tags []string,
		koboSync bool,
	) error
	UpdateUserBookAddedAt(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		addedAt time.Time,
	) error
	UpsertBook(ctx context.Context, book models.Book) (*models.Book, error)
	UpsertKoboRemoval(ctx context.Context, userID string, bookID uuid.UUID) error
	UpsertKoboStoreBook(
		ctx context.Context,
		userID string,
		sb models.KoboStoreBook,
	) error
	UpsertUserBook(ctx context.Context, ub models.UserBook) error
}

// BookFileStore is the stored-file storage BookService uses.
type BookFileStore interface {
	CountByStorageKey(ctx context.Context, storageKey string) (int64, error)
	DeleteByUserBook(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) (int64, error)
	FilesByUser(
		ctx context.Context,
		userID string,
	) (map[uuid.UUID]models.LibraryFiles, error)
	FindByChecksum(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		format string,
		checksum string,
	) (*models.BookFile, error)
	FindByChecksumForUser(
		ctx context.Context,
		userID string,
		checksum string,
	) (*models.BookFile, error)
	FindByChecksumGlobal(ctx context.Context, checksum string) (*models.BookFile, error)
	GetByBookAndFormat(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		format string,
	) (*models.BookFile, error)
	Insert(ctx context.Context, f models.BookFile) (*models.BookFile, error)
	ListByBook(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) ([]models.BookFile, error)
	RepointAndDedup(
		ctx context.Context,
		userID string,
		fromBookID uuid.UUID,
		toBookID uuid.UUID,
	) ([]string, error)
}

// ReadingStateStore is the per-user reading-state storage BookService uses.
type ReadingStateStore interface {
	ClearKoboLocation(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		loc models.KoboLocation,
	) error
	DeleteByBook(ctx context.Context, userID string, bookID uuid.UUID) error
	Get(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
	) (*models.BookReadingState, error)
	ListByUser(ctx context.Context, userID string) ([]models.BookReadingState, error)
	ListKoboDeviceHeld(
		ctx context.Context,
		deviceID string,
	) (map[uuid.UUID]time.Time, error)
	MarkKoboDeviceHeld(
		ctx context.Context,
		deviceID string,
		bookIDs []uuid.UUID,
		updatedAts []time.Time,
	) error
	SetPositionForKoboLocation(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		loc models.KoboLocation,
		pos models.ReadingPosition,
	) error
	Upsert(ctx context.Context, state models.BookReadingState) error
	UpsertIfNewer(
		ctx context.Context,
		state models.BookReadingState,
	) (*time.Time, error)
}

// ingestStore is the content storage IngestService writes backfilled articles to.
type ingestStore interface {
	SetBookContentHTML(ctx context.Context, bookID uuid.UUID, html string) error
}

// CoverFetcher downloads a cover image from a URL that can be user-supplied.
// It fails on a non-2xx response or a body over maxBytes.
type CoverFetcher interface {
	Fetch(ctx context.Context, url string, maxBytes int) ([]byte, error)
}
