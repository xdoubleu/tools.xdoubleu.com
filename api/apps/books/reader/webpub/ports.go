package webpub

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// bookFileStore is the stored-file lookup the webpub service reads.
type bookFileStore interface {
	GetByBookAndFormat(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		format string,
	) (*models.BookFile, error)
}
