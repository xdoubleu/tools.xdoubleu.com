package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

type accessConnectHandler struct {
	app *Application
}

var _ accessv1connect.AccessServiceHandler = (*accessConnectHandler)(nil)

// requireAdmin gates admin-only Connect RPCs. It is shared by the access and
// observability handlers, both of which are registered behind admin auth.
func requireAdmin(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin {
		return connect.NewError(
			connect.CodePermissionDenied,
			errors.New("admin access required"),
		)
	}
	return nil
}

func protoAppUser(u models.User) *accessv1.AppUser {
	access := u.AppAccess
	if access == nil {
		access = []string{}
	}
	return &accessv1.AppUser{
		Id:        u.ID,
		Email:     u.Email,
		Role:      string(u.Role),
		AppAccess: access,
	}
}

func (h *accessConnectHandler) ListUsers(
	ctx context.Context,
	_ *connect.Request[accessv1.ListUsersRequest],
) (*connect.Response[accessv1.ListUsersResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := h.app.appUsersRepo.GetAllWithAccess(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoUsers := make([]*accessv1.AppUser, len(users))
	for i, u := range users {
		protoUsers[i] = protoAppUser(u)
	}

	return connect.NewResponse(&accessv1.ListUsersResponse{Users: protoUsers}), nil
}

func (h *accessConnectHandler) SetRole(
	ctx context.Context,
	req *connect.Request[accessv1.SetRoleRequest],
) (*connect.Response[accessv1.SetRoleResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	role := models.Role(req.Msg.Role)
	if role != models.RoleAdmin && role != models.RoleUser {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid role"),
		)
	}

	if err := h.app.appUsersRepo.SetRole(ctx, req.Msg.UserId, role); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.app.auth.InvalidateUserCache()

	user, err := h.app.appUsersRepo.GetByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(
		&accessv1.SetRoleResponse{User: protoAppUser(*user)},
	), nil
}

func (h *accessConnectHandler) SetAppAccess(
	ctx context.Context,
	req *connect.Request[accessv1.SetAppAccessRequest],
) (*connect.Response[accessv1.SetAppAccessResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.app.appUsersRepo.SetAppAccess(
		ctx, req.Msg.UserId, req.Msg.AppName, req.Msg.Grant,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.app.auth.InvalidateUserCache()

	user, err := h.app.appUsersRepo.GetByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(
		&accessv1.SetAppAccessResponse{User: protoAppUser(*user)},
	), nil
}
