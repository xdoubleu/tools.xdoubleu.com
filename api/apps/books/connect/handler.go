package connect

import (
	"context"
	"log/slog"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/books/internal/protomap"
	"tools.xdoubleu.com/apps/books/internal/services"
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

func mapError(err error, extra ...connecttools.ErrorCode) error {
	return protomap.MapError(err, extra...)
}

// mapISBNError names the clash when an ISBN is already taken.
func mapISBNError(err error) error {
	return mapError(err, connecttools.CodeWithMessage(
		database.ErrResourceConflict,
		connect.CodeAlreadyExists,
		"ISBN is already assigned to another book",
	))
}
