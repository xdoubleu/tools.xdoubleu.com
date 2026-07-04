package todos

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

func (h *settingsConnectHandler) CreateSection(
	ctx context.Context,
	req *connect.Request[todosv1.CreateSectionRequest],
) (*connect.Response[todosv1.CreateSectionResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	dto := dtos.CreateSectionDto{Name: req.Msg.Name}
	if _, err = h.app.services.Sections.Create(ctx, userID, dto, workspaceID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.CreateSectionResponse{}), nil
}

func (h *settingsConnectHandler) DeleteSection(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteSectionRequest],
) (*connect.Response[todosv1.DeleteSectionResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Sections.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteSectionResponse{}), nil
}

func (h *settingsConnectHandler) CreatePolicy(
	ctx context.Context,
	req *connect.Request[todosv1.CreatePolicyRequest],
) (*connect.Response[todosv1.CreatePolicyResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	if _, err = h.app.services.Policies.Create(
		ctx, userID, req.Msg.Text, int(req.Msg.ReappearAfterHours), workspaceID,
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.CreatePolicyResponse{}), nil
}

func (h *settingsConnectHandler) UpdatePolicy(
	ctx context.Context,
	req *connect.Request[todosv1.UpdatePolicyRequest],
) (*connect.Response[todosv1.UpdatePolicyResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if _, err = h.app.services.Policies.Update(
		ctx, id, userID, req.Msg.Text, int(req.Msg.ReappearAfterHours),
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdatePolicyResponse{}), nil
}

func (h *settingsConnectHandler) DeletePolicy(
	ctx context.Context,
	req *connect.Request[todosv1.DeletePolicyRequest],
) (*connect.Response[todosv1.DeletePolicyResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Policies.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeletePolicyResponse{}), nil
}

func (h *settingsConnectHandler) CreateWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.CreateWorkspaceRequest],
) (*connect.Response[todosv1.CreateWorkspaceResponse], error) {
	userID := h.userID(ctx)
	if _, err := h.app.services.Workspaces.Create(ctx, userID, req.Msg.Name); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.CreateWorkspaceResponse{}), nil
}

func (h *settingsConnectHandler) DeleteWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteWorkspaceRequest],
) (*connect.Response[todosv1.DeleteWorkspaceResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Workspaces.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteWorkspaceResponse{}), nil
}

func (h *settingsConnectHandler) SetActiveWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.SetActiveWorkspaceRequest],
) (*connect.Response[todosv1.SetActiveWorkspaceResponse], error) {
	userID := h.userID(ctx)
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if err := h.app.services.Settings.SetActiveWorkspace(
		ctx, userID, workspaceID,
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.SetActiveWorkspaceResponse{}), nil
}
