package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	adminv1 "tools.xdoubleu.com/gen/admin/v1"
	"tools.xdoubleu.com/gen/admin/v1/adminv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

type adminConnectHandler struct {
	app *Application
}

var _ adminv1connect.AdminServiceHandler = (*adminConnectHandler)(nil)

func (h *adminConnectHandler) requireAdmin(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin {
		return connect.NewError(
			connect.CodePermissionDenied,
			errors.New("admin access required"),
		)
	}
	return nil
}

func protoAppUser(u models.User) *adminv1.AppUser {
	access := u.AppAccess
	if access == nil {
		access = []string{}
	}
	return &adminv1.AppUser{
		Id:        u.ID,
		Email:     u.Email,
		Role:      string(u.Role),
		AppAccess: access,
	}
}

func (h *adminConnectHandler) ListUsers(
	ctx context.Context,
	_ *connect.Request[adminv1.ListUsersRequest],
) (*connect.Response[adminv1.ListUsersResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := h.app.appUsersRepo.GetAllWithAccess(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoUsers := make([]*adminv1.AppUser, len(users))
	for i, u := range users {
		protoUsers[i] = protoAppUser(u)
	}

	return connect.NewResponse(&adminv1.ListUsersResponse{Users: protoUsers}), nil
}

func (h *adminConnectHandler) SetRole(
	ctx context.Context,
	req *connect.Request[adminv1.SetRoleRequest],
) (*connect.Response[adminv1.SetRoleResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
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

	user, err := h.app.appUsersRepo.GetByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&adminv1.SetRoleResponse{User: protoAppUser(*user)}), nil
}

func (h *adminConnectHandler) SetAppAccess(
	ctx context.Context,
	req *connect.Request[adminv1.SetAppAccessRequest],
) (*connect.Response[adminv1.SetAppAccessResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.app.appUsersRepo.SetAppAccess(
		ctx, req.Msg.UserId, req.Msg.AppName, req.Msg.Grant,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	user, err := h.app.appUsersRepo.GetByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(
		&adminv1.SetAppAccessResponse{User: protoAppUser(*user)},
	), nil
}
