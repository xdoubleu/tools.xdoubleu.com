package connect

import (
	"context"
	"log/slog"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/reader/positions"
	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var (
	_ booksv1connect.LibraryServiceHandler   = (*Handler)(nil)
	_ booksv1connect.BookFilesServiceHandler = (*Handler)(nil)
	_ booksv1connect.KoboServiceHandler      = (*Handler)(nil)
	_ booksv1connect.CatalogServiceHandler   = (*Handler)(nil)
)

// Handler serves the books library, files, Kobo and catalog services.
type Handler struct {
	services     *services.Services
	logger       *slog.Logger
	coverBaseURL string
}

// NewHandler returns a Handler backed by the books services; coverBaseURL
// prefixes the cover URLs it returns.
func NewHandler(
	s *services.Services,
	logger *slog.Logger,
	coverBaseURL string,
) *Handler {
	return &Handler{services: s, logger: logger, coverBaseURL: coverBaseURL}
}

// caller returns the authenticated user, or nil.
func caller(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

// MapError translates a books service error into a ConnectRPC error.
func MapError(err error) error {
	return mapError(err)
}

func mapError(err error, extra ...connecttools.ErrorCode) error {
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

// mapISBNError names the clash when an ISBN is already taken.
func mapISBNError(err error) error {
	return mapError(err, connecttools.CodeWithMessage(
		database.ErrResourceConflict,
		connect.CodeAlreadyExists,
		"ISBN is already assigned to another book",
	))
}
