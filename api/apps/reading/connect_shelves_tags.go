package reading

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) ToggleTag(
	ctx context.Context,
	req *connect.Request[readingv1.ToggleTagRequest],
) (*connect.Response[readingv1.ToggleTagResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	if req.Msg.Tag == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("tag cannot be empty"),
		)
	}
	err = h.app.Services.Books.ToggleTag(ctx, user.ID, bookID, req.Msg.Tag)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.ToggleTagResponse{}), nil
}

func (h *booksConnectHandler) CreateShelf(
	ctx context.Context,
	req *connect.Request[readingv1.CreateShelfRequest],
) (*connect.Response[readingv1.CreateShelfResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	err := h.app.Services.Books.CreateShelf(ctx, user.ID, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&readingv1.CreateShelfResponse{}), nil
}

func (h *booksConnectHandler) RenameShelf(
	ctx context.Context,
	req *connect.Request[readingv1.RenameShelfRequest],
) (*connect.Response[readingv1.RenameShelfResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	moved, err := h.app.Services.Books.RenameShelf(
		ctx, user.ID, req.Msg.OldName, req.Msg.NewName,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&readingv1.RenameShelfResponse{Moved: moved}), nil
}

func (h *booksConnectHandler) DeleteShelf(
	ctx context.Context,
	req *connect.Request[readingv1.DeleteShelfRequest],
) (*connect.Response[readingv1.DeleteShelfResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	moved, err := h.app.Services.Books.DeleteShelf(
		ctx, user.ID, req.Msg.Name, req.Msg.TargetName,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&readingv1.DeleteShelfResponse{Moved: moved}), nil
}

func (h *booksConnectHandler) RenameTag(
	ctx context.Context,
	req *connect.Request[readingv1.RenameTagRequest],
) (*connect.Response[readingv1.RenameTagResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	affected, err := h.app.Services.Books.RenameTag(
		ctx, user.ID, req.Msg.OldName, req.Msg.NewName,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&readingv1.RenameTagResponse{Affected: affected}), nil
}

func (h *booksConnectHandler) DeleteTag(
	ctx context.Context,
	req *connect.Request[readingv1.DeleteTagRequest],
) (*connect.Response[readingv1.DeleteTagResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	affected, err := h.app.Services.Books.DeleteTag(ctx, user.ID, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&readingv1.DeleteTagResponse{Affected: affected}), nil
}
