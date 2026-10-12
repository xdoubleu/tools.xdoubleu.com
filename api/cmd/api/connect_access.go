package main

import (
	"context"

	"connectrpc.com/connect/v2"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

type accessConnectHandler struct {
	app *Application
}

var _ accessv1connect.AccessServiceHandler = (*accessConnectHandler)(nil)

// requireAdmin gates admin-only Connect RPCs.
func requireAdmin(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return nil
}

// requireObservability gates the observability MCP tools: admins and the
// service role, which has no access to user data.
func requireObservability(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin && user.Role != models.RoleService {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
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
	_ *accessv1.ListUsersRequest,
) (*accessv1.ListUsersResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := h.app.appUsersRepo.GetAllWithAccess(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	protoUsers := make([]*accessv1.AppUser, len(users))
	for i, u := range users {
		protoUsers[i] = protoAppUser(u)
	}

	return &accessv1.ListUsersResponse{Users: protoUsers}, nil
}

func (h *accessConnectHandler) SetRole(
	ctx context.Context,
	req *accessv1.SetRoleRequest,
) (*accessv1.SetRoleResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	role := models.Role(req.Role)
	if role != models.RoleAdmin && role != models.RoleUser {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid role",
		)
	}

	if err := h.app.appUsersRepo.SetRole(ctx, req.UserId, role); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	h.app.auth.InvalidateUserCache()

	user, err := h.app.appUsersRepo.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &accessv1.SetRoleResponse{User: protoAppUser(*user)}, nil
}

func (h *accessConnectHandler) SetAppAccess(
	ctx context.Context,
	req *accessv1.SetAppAccessRequest,
) (*accessv1.SetAppAccessResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.app.appUsersRepo.SetAppAccess(
		ctx, req.UserId, req.AppName, req.Grant,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	h.app.auth.InvalidateUserCache()

	user, err := h.app.appUsersRepo.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &accessv1.SetAppAccessResponse{User: protoAppUser(*user)}, nil
}
