package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *Handler) ToggleTag(
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
	err = h.services.Books.ToggleTag(ctx, user.ID, bookID, req.Tag)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.ToggleTagResponse{}, nil
}

func (h *Handler) SetBookTag(
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
	err = h.services.Books.SetTag(
		ctx, user.ID, bookID, req.Tag, req.Enabled,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.SetBookTagResponse{}, nil
}

func (h *Handler) CreateShelf(
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
	err := h.services.Books.CreateShelf(ctx, user.ID, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.CreateShelfResponse{}, nil
}

func (h *Handler) RenameShelf(
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
	moved, err := h.services.Books.RenameShelf(
		ctx, user.ID, req.OldName, req.NewName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RenameShelfResponse{Moved: moved}, nil
}

func (h *Handler) DeleteShelf(
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
	moved, err := h.services.Books.DeleteShelf(
		ctx, user.ID, req.Name, req.TargetName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.DeleteShelfResponse{Moved: moved}, nil
}

func (h *Handler) RenameTag(
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
	affected, err := h.services.Books.RenameTag(
		ctx, user.ID, req.OldName, req.NewName,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RenameTagResponse{Affected: affected}, nil
}

func (h *Handler) DeleteTag(
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
	affected, err := h.services.Books.DeleteTag(ctx, user.ID, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.DeleteTagResponse{Affected: affected}, nil
}
