package account

import (
	"context"

	"connectrpc.com/connect/v2"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	accessv1connect "tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/obs"
)

type accessConnectHandler struct {
	app *Deps
}

var _ accessv1connect.AccessServiceHandler = (*accessConnectHandler)(nil)

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
	if err := obs.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := h.app.AppUsers.GetAllWithAccess(ctx)
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
	if err := obs.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	role := models.Role(req.Role)
	if role != models.RoleAdmin && role != models.RoleUser {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid role",
		)
	}

	if err := h.app.AppUsers.SetRole(ctx, req.UserId, role); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	h.app.Auth.InvalidateUserCache()

	user, err := h.app.AppUsers.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &accessv1.SetRoleResponse{User: protoAppUser(*user)}, nil
}

func (h *accessConnectHandler) SetAppAccess(
	ctx context.Context,
	req *accessv1.SetAppAccessRequest,
) (*accessv1.SetAppAccessResponse, error) {
	if err := obs.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.app.AppUsers.SetAppAccess(
		ctx, req.UserId, req.AppName, req.Grant,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	h.app.Auth.InvalidateUserCache()

	user, err := h.app.AppUsers.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &accessv1.SetAppAccessResponse{User: protoAppUser(*user)}, nil
}
