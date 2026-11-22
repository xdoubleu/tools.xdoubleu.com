package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Errors the catalog rules return; adapters map them to transport codes.
var (
	ErrUnauthenticated  = errors.New("unauthorized")
	ErrAdminRequired    = errors.New("admin access required")
	ErrMetadataRequired = errors.New("metadata is required")
	ErrISBNLength       = errors.New("isbn13 must be exactly 13 digits")
	ErrISBNDigits       = errors.New("isbn13 must contain only digits")

	// ErrUnknownSources is returned for a source name the catalog doesn't
	// track.
	ErrUnknownSources = repositories.ErrUnknownSources
)

const isbn13Length = 13

// ResyncJob controls the background catalog resync.
type ResyncJob interface {
	// Start arms the job (force re-checks every book) and runs it now.
	Start(force bool)
	Cancel()
}

// SourceStats aggregates per-source coverage, uniqueness and overlap. A
// source counts as absent only when checked and empty, never when unresolved.
type SourceStats struct {
	TotalBooks     int
	UniCatFound    int
	HardcoverFound int
	// Unique: found here and confirmed absent from the other.
	UniCatUnique    int
	HardcoverUnique int
	// Missed: checked and empty, distinct from never scanned.
	UniCatMissed    int
	HardcoverMissed int
	// BothMissed requires both explicitly empty, unlike NotFoundAnywhere.
	Both             int
	BothMissed       int
	NotFoundAnywhere int
	NeverScanned     int
}

// MergeRequest merges Losers into Winner, optionally overriding the winner's
// metadata, cover source and status.
type MergeRequest struct {
	Winner      uuid.UUID
	Losers      []uuid.UUID
	Metadata    *models.Book
	CoverSource *uuid.UUID
	Status      *string
}

// CatalogService holds the catalog and admin rules: who may run them, what
// they rebuild afterwards, and the validation of admin edits.
type CatalogService struct {
	logger   *slog.Logger
	books    *BookService
	progress *ProgressService
	resync   ResyncJob
}

// NewCatalogService returns a CatalogService over the book and progress
// services; resync controls the background resync job.
func NewCatalogService(
	logger *slog.Logger,
	books *BookService,
	progress *ProgressService,
	resync ResyncJob,
) *CatalogService {
	return &CatalogService{
		logger:   logger,
		books:    books,
		progress: progress,
		resync:   resync,
	}
}

// RequireAdmin returns ErrUnauthenticated for no caller and ErrAdminRequired
// for a non-admin one.
func RequireAdmin(caller *sharedmodels.User) error {
	if caller == nil {
		return ErrUnauthenticated
	}
	if caller.Role != sharedmodels.RoleAdmin {
		return ErrAdminRequired
	}
	return nil
}

// RebuildReadProgress recomputes and stores userID's reading-progress graph.
func (s *CatalogService) RebuildReadProgress(
	ctx context.Context,
	userID string,
) error {
	labels, values, err := s.books.BuildReadProgress(ctx, userID)
	if err != nil {
		return err
	}
	return s.progress.Save(ctx, userID, labels, values)
}

// ImportBooks upserts a Goodreads CSV export into userID's library and
// returns the imported count.
func (s *CatalogService) ImportBooks(
	ctx context.Context,
	userID string,
	r io.Reader,
) (int, error) {
	count, err := s.books.ImportFromCSV(ctx, userID, r)
	if err != nil {
		return 0, err
	}
	return count, s.RebuildReadProgress(ctx, userID)
}

// FindDuplicates lists library entries judged to be the same book.
func (s *CatalogService) FindDuplicates(
	ctx context.Context,
	caller *sharedmodels.User,
) ([]DuplicateGroup, error) {
	if err := RequireAdmin(caller); err != nil {
		return nil, err
	}
	return s.books.FindDuplicates(ctx, caller.ID)
}

// MergeBooks merges the losers into the winner, rebuilds the reading progress
// of every affected user, and returns the number of deleted files.
func (s *CatalogService) MergeBooks(
	ctx context.Context,
	caller *sharedmodels.User,
	req MergeRequest,
) (uint32, error) {
	if err := RequireAdmin(caller); err != nil {
		return 0, err
	}
	deletedFiles, affectedUsers, err := s.books.MergeBooks(
		ctx,
		caller.ID,
		req.Winner,
		req.Losers,
		req.Metadata,
		req.CoverSource,
		req.Status,
	)
	if err != nil {
		return 0, err
	}
	for _, uid := range affectedUsers {
		if err = s.RebuildReadProgress(ctx, uid); err != nil {
			return 0, err
		}
	}
	return deletedFiles, nil
}

// StartResync runs the catalog resync now.
func (s *CatalogService) StartResync(
	caller *sharedmodels.User,
	force bool,
) error {
	if err := RequireAdmin(caller); err != nil {
		return err
	}
	s.resync.Start(force)
	return nil
}

// CancelResync stops a running catalog resync.
func (s *CatalogService) CancelResync(caller *sharedmodels.User) error {
	if err := RequireAdmin(caller); err != nil {
		return err
	}
	s.resync.Cancel()
	return nil
}

// ListResyncProposals lists catalog books that differ from an external
// source.
func (s *CatalogService) ListResyncProposals(
	ctx context.Context,
	caller *sharedmodels.User,
) ([]ResyncProposal, error) {
	if err := RequireAdmin(caller); err != nil {
		return nil, err
	}
	return s.books.ListResyncProposals(ctx)
}

// ApplyResyncChoice applies the chosen source's proposal to the book.
func (s *CatalogService) ApplyResyncChoice(
	ctx context.Context,
	caller *sharedmodels.User,
	bookID uuid.UUID,
	source string,
) error {
	if err := RequireAdmin(caller); err != nil {
		return err
	}
	return s.books.ApplyResyncChoice(ctx, s.logger, bookID, source)
}

// GetBookSources live-fetches the book's candidates from every source.
func (s *CatalogService) GetBookSources(
	ctx context.Context,
	caller *sharedmodels.User,
	bookID uuid.UUID,
	overrideTitle, overrideAuthor string,
) (ResyncProposal, error) {
	if err := RequireAdmin(caller); err != nil {
		return ResyncProposal{}, err
	}
	return s.books.GetBookSources(
		ctx, s.logger, bookID, overrideTitle, overrideAuthor,
	)
}

// ApplyBookSource syncs the book from one candidate of a source.
func (s *CatalogService) ApplyBookSource(
	ctx context.Context,
	caller *sharedmodels.User,
	bookID uuid.UUID,
	source string,
	index int,
	overrideTitle, overrideAuthor string,
) error {
	if err := RequireAdmin(caller); err != nil {
		return err
	}
	return s.books.SyncBookSource(
		ctx, s.logger, bookID, source, index, overrideTitle, overrideAuthor,
	)
}

// SetBookISBN assigns a validated ISBN-13 to the book.
func (s *CatalogService) SetBookISBN(
	ctx context.Context,
	caller *sharedmodels.User,
	bookID uuid.UUID,
	isbn13 string,
) error {
	if err := RequireAdmin(caller); err != nil {
		return err
	}
	normalized, err := NormalizeISBN13(isbn13)
	if err != nil {
		return err
	}
	return s.books.SetBookISBN(ctx, bookID, normalized)
}

// UpdateBook hand-corrects a catalog book's metadata.
func (s *CatalogService) UpdateBook(
	ctx context.Context,
	caller *sharedmodels.User,
	bookID uuid.UUID,
	metadata *models.Book,
	coverURL string,
) (*models.Book, error) {
	if err := RequireAdmin(caller); err != nil {
		return nil, err
	}
	if metadata == nil {
		return nil, ErrMetadataRequired
	}
	edited := *metadata
	if edited.ISBN13 != nil {
		normalized, err := NormalizeISBN13(*edited.ISBN13)
		if err != nil {
			return nil, err
		}
		edited.ISBN13 = &normalized
	}
	return s.books.UpdateBook(ctx, bookID, edited, coverURL)
}

// GetSourceStats reports per-source metadata coverage over the catalog.
func (s *CatalogService) GetSourceStats(
	ctx context.Context,
	caller *sharedmodels.User,
) (*SourceStats, error) {
	if err := RequireAdmin(caller); err != nil {
		return nil, err
	}
	return s.books.GetSourceStats(ctx)
}

// ListBooksInExactSources lists catalog books found by exactly the given
// sources.
func (s *CatalogService) ListBooksInExactSources(
	ctx context.Context,
	caller *sharedmodels.User,
	sources []string,
) ([]models.Book, error) {
	if err := RequireAdmin(caller); err != nil {
		return nil, err
	}
	return s.books.ListBooksInExactSources(ctx, sources)
}

// NormalizeISBN13 strips spaces and hyphens and requires exactly 13 digits.
func NormalizeISBN13(raw string) (string, error) {
	normalized := strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, raw)
	if len(normalized) != isbn13Length {
		return "", ErrISBNLength
	}
	for _, r := range normalized {
		if r < '0' || r > '9' {
			return "", ErrISBNDigits
		}
	}
	return normalized, nil
}
