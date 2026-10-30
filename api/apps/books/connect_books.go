package books

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/internal/services/positions"
	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var (
	_ booksv1connect.LibraryServiceHandler   = (*booksConnectHandler)(nil)
	_ booksv1connect.BookFilesServiceHandler = (*booksConnectHandler)(nil)
	_ booksv1connect.KoboServiceHandler      = (*booksConnectHandler)(nil)
	_ booksv1connect.CatalogServiceHandler   = (*booksConnectHandler)(nil)
)

type booksConnectHandler struct {
	app *Books
}

// requireAdmin returns an authenticated admin user or a PermissionDenied error.
func (h *booksConnectHandler) requireAdmin(
	ctx context.Context,
) (*sharedmodels.User, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	if user.Role != sharedmodels.RoleAdmin {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return user, nil
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrProposalNotFound, connect.CodeNotFound),
		connecttools.Code(services.ErrExternalNotFound, connect.CodeNotFound),
		connecttools.Code(services.ErrDerivedTag, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrEmptyTag, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidShelf, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidReadingSource, connect.CodeInvalidArgument),
		connecttools.Code(positions.ErrInvalidPhysicalCopy, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidFormat, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrUnrecognizedBook, connect.CodeInvalidArgument),
		connecttools.Code(repositories.ErrUnknownSources, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrFileTooLarge, connect.CodeResourceExhausted),
		connecttools.Code(services.ErrInvalidUploadID, connect.CodePermissionDenied),
		connecttools.Code(services.ErrUploadMissing, connect.CodeFailedPrecondition),
	)
}

// mapISBNError names the clash when an ISBN is already taken.
func mapISBNError(err error) error {
	return connecttools.MapError(err, connecttools.CodeWithMessage(
		database.ErrResourceConflict,
		connect.CodeAlreadyExists,
		"ISBN is already assigned to another book",
	))
}
