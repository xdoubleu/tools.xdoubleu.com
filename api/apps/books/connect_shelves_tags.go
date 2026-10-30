package books

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) ToggleTag(
	ctx context.Context,
	req *booksv1.ToggleTagRequest,
) (*booksv1.ToggleTagResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	if req.Tag == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"tag cannot be empty",
		)
	}
	err = h.app.services.Books.ToggleTag(ctx, user.ID, bookID, req.Tag)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.ToggleTagResponse{}, nil
}

func (h *booksConnectHandler) SetBookTag(
	ctx context.Context,
	req *booksv1.SetBookTagRequest,
) (*booksv1.SetBookTagResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	if req.Tag == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"tag cannot be empty",
		)
	}
	err = h.app.services.Books.SetTag(
		ctx, user.ID, bookID, req.Tag, req.Enabled,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.SetBookTagResponse{}, nil
}

func (h *booksConnectHandler) CreateShelf(
	ctx context.Context,
	req *booksv1.CreateShelfRequest,
) (*booksv1.CreateShelfResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	err := h.app.services.Books.CreateShelf(ctx, user.ID, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.CreateShelfResponse{}, nil
}

func (h *booksConnectHandler) RenameShelf(
	ctx context.Context,
	req *booksv1.RenameShelfRequest,
) (*booksv1.RenameShelfResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	moved, err := h.app.services.Books.RenameShelf(
		ctx, user.ID, req.OldName, req.NewName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RenameShelfResponse{Moved: moved}, nil
}

func (h *booksConnectHandler) DeleteShelf(
	ctx context.Context,
	req *booksv1.DeleteShelfRequest,
) (*booksv1.DeleteShelfResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	moved, err := h.app.services.Books.DeleteShelf(
		ctx, user.ID, req.Name, req.TargetName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.DeleteShelfResponse{Moved: moved}, nil
}

func (h *booksConnectHandler) RenameTag(
	ctx context.Context,
	req *booksv1.RenameTagRequest,
) (*booksv1.RenameTagResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	affected, err := h.app.services.Books.RenameTag(
		ctx, user.ID, req.OldName, req.NewName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RenameTagResponse{Affected: affected}, nil
}

func (h *booksConnectHandler) DeleteTag(
	ctx context.Context,
	req *booksv1.DeleteTagRequest,
) (*booksv1.DeleteTagResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	affected, err := h.app.services.Books.DeleteTag(ctx, user.ID, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.DeleteTagResponse{Affected: affected}, nil
}
