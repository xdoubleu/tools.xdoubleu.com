package protomap

import (
	"fmt"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/reader/positions"
	"tools.xdoubleu.com/internal/connecttools"
)

// MapError translates a books service error into a ConnectRPC error.
func MapError(err error, extra ...connecttools.ErrorCode) error {
	codes := append([]connecttools.ErrorCode{}, extra...)
	codes = append(codes,
		connecttools.Code(services.ErrProposalNotFound, connect.CodeNotFound),
		connecttools.Code(services.ErrExternalNotFound, connect.CodeNotFound),
		connecttools.Code(services.ErrDerivedTag, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrEmptyTag, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidShelf, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidReadingSource, connect.CodeInvalidArgument),
		connecttools.Code(positions.ErrInvalidPhysicalCopy, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidFormat, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrUnrecognizedBook, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrUnknownSources, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrFileTooLarge, connect.CodeResourceExhausted),
		connecttools.Code(services.ErrInvalidUploadID, connect.CodePermissionDenied),
		connecttools.Code(services.ErrUploadMissing, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrUnauthenticated, connect.CodeUnauthenticated),
		connecttools.Code(services.ErrAdminRequired, connect.CodePermissionDenied),
		connecttools.Code(services.ErrMetadataRequired, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrISBNLength, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrISBNDigits, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrSeriesNameRequired, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidDeviceID, connect.CodeInvalidArgument),
	)
	return connecttools.MapError(err, codes...)
}

// ParseBookID parses a book_id request field.
func ParseBookID(rawBookID string) (uuid.UUID, error) {
	bookID, err := uuid.Parse(rawBookID)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid book_id: %w", err).Error(),
		)
	}
	return bookID, nil
}
