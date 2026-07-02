package backlog

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ backlogv1connect.BooksServiceHandler = (*booksConnectHandler)(nil)

type booksConnectHandler struct {
	app *Backlog
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
