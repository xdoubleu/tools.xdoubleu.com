package conversion

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
)

// bookStore is the book storage the conversion service reads.
type bookStore interface {
	GetBookByID(ctx context.Context, bookID uuid.UUID) (*models.Book, error)
}

// bookFileStore is the stored-file storage conversion reads and updates.
type bookFileStore interface {
	CopyFigureStats(ctx context.Context) (int64, error)
	FindByStorageKeyGlobal(
		ctx context.Context,
		storageKey string,
	) (*models.BookFile, error)
	GetByBookAndFormat(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		format string,
	) (*models.BookFile, error)
	Insert(ctx context.Context, f models.BookFile) (*models.BookFile, error)
	ListUnanalysed(
		ctx context.Context,
		limit int,
	) ([]models.FigureAnalysisTarget, error)
	Replace(
		ctx context.Context,
		staleID uuid.UUID,
		f models.BookFile,
	) (*models.BookFile, error)
	SetFigureStats(
		ctx context.Context,
		storageKey string,
		stats models.FigureStats,
	) error
	UpdateAfterConversion(
		ctx context.Context,
		id uuid.UUID,
		storageKey string,
		sizeBytes int64,
		converterVersion int16,
	) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// kepubRequestStore is the on-demand KEPUB conversion queue storage.
type kepubRequestStore interface {
	Delete(ctx context.Context, userID string, bookID uuid.UUID) error
	Enqueue(ctx context.Context, userID string, bookID uuid.UUID) error
	ListOldest(ctx context.Context, limit int) ([]repositories.KEPUBRequest, error)
}
