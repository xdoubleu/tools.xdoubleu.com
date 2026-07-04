package books

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	"tools.xdoubleu.com/internal/constants"
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
			errors.New("unauthorized"),
		)
	}
	if user.Role != sharedmodels.RoleAdmin {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			errors.New("admin access required"),
		)
	}
	return user, nil
}
