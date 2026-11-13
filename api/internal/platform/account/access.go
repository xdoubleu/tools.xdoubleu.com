package account

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect/v2"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	accessv1connect "tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/obs"
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
	write := u.WriteAccess
	if write == nil {
		write = []string{}
	}
	return &accessv1.AppUser{
		Id:          u.ID,
		Email:       u.Email,
		Role:        string(u.Role),
		AppAccess:   access,
		WriteAccess: write,
		DisplayName: u.DisplayName,
	}
}

// SetAgentAppAccess is the only way to change what a standing agent reaches;
// SetAppAccess and SetRole keep refusing agents (adr-0047).
func (h *accessConnectHandler) SetAgentAppAccess(
	ctx context.Context,
	req *accessv1.SetAgentAppAccessRequest,
) (*accessv1.SetAgentAppAccessResponse, error) {
	if err := obs.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	switch req.Level {
	case AgentRead, AgentWrite, AgentNoAccess:
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid level")
	}
	if req.AppName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "app_name required")
	}

	ok, err := h.app.AppUsers.SetAgentAppAccess(
		ctx, req.UserId, req.AppName, req.Level,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, "agent account not found")
	}
	h.app.Auth.InvalidateUserCache()

	user, err := h.app.AppUsers.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &accessv1.SetAgentAppAccessResponse{User: protoAppUser(*user)}, nil
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

// refuseAgent denies a role or access change to an agent account: its
// identity is fixed to the factory tools.
func (h *accessConnectHandler) refuseAgent(ctx context.Context, userID string) error {
	target, err := h.app.AppUsers.GetByID(ctx, userID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, "user not found").WithCause(err)
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	if target.Role == models.RoleAgent {
		return connect.NewError(
			connect.CodePermissionDenied, "agent accounts can't be changed here",
		)
	}
	return nil
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

	if err := h.refuseAgent(ctx, req.UserId); err != nil {
		return nil, err
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

	if err := h.refuseAgent(ctx, req.UserId); err != nil {
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

func (h *accessConnectHandler) SetDisplayName(
	ctx context.Context,
	req *accessv1.SetDisplayNameRequest,
) (*accessv1.SetDisplayNameResponse, error) {
	if err := obs.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"display_name is required",
		)
	}

	if err := h.refuseAgent(ctx, req.UserId); err != nil {
		return nil, err
	}
	if err := h.app.AppUsers.SetDisplayName(ctx, req.UserId, name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	user, err := h.app.AppUsers.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &accessv1.SetDisplayNameResponse{User: protoAppUser(*user)}, nil
}
