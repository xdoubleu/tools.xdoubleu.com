package todos

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

func (h *settingsConnectHandler) CreateLabelPreset(
	ctx context.Context,
	req *connect.Request[todosv1.CreateLabelPresetRequest],
) (*connect.Response[todosv1.CreateLabelPresetResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	dto := dtos.CreateLabelPresetDto{
		Category: req.Msg.Category,
		Value:    req.Msg.Value,
	}
	if addErr := h.app.services.Settings.CreateLabelPreset(
		ctx, userID, dto, workspaceID,
	); addErr != nil {
		return nil, connectErr(addErr)
	}
	return connect.NewResponse(&todosv1.CreateLabelPresetResponse{}), nil
}

func (h *settingsConnectHandler) DeleteLabelPreset(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteLabelPresetRequest],
) (*connect.Response[todosv1.DeleteLabelPresetResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	if removeErr := h.app.services.Settings.DeleteLabelPreset(
		ctx, userID, req.Msg.Category, req.Msg.Value, workspaceID,
	); removeErr != nil {
		return nil, connectErr(removeErr)
	}
	return connect.NewResponse(&todosv1.DeleteLabelPresetResponse{}), nil
}

func (h *settingsConnectHandler) UpdateLabelColor(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateLabelColorRequest],
) (*connect.Response[todosv1.UpdateLabelColorResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	if updateErr := h.app.services.Settings.UpdateLabelColor(
		ctx, userID, req.Msg.Category, req.Msg.Value, workspaceID,
		req.Msg.Color,
	); updateErr != nil {
		return nil, connectErr(updateErr)
	}
	return connect.NewResponse(&todosv1.UpdateLabelColorResponse{}), nil
}

func (h *settingsConnectHandler) CreateURLPattern(
	ctx context.Context,
	req *connect.Request[todosv1.CreateURLPatternRequest],
) (*connect.Response[todosv1.CreateURLPatternResponse], error) {
	userID := h.userID(ctx)
	workspaceID, err := h.resolveWorkspaceID(ctx, userID, req.Msg.WorkspaceId)
	if err != nil {
		return nil, connectErr(err)
	}
	dto := dtos.CreateURLPatternDto{
		URLPrefix:    req.Msg.UrlPrefix,
		PlatformName: req.Msg.PlatformName,
		Label:        req.Msg.Label,
		Shortcut:     req.Msg.Shortcut,
	}
	if addErr := h.app.services.Settings.CreateURLPattern(
		ctx, userID, dto, workspaceID,
	); addErr != nil {
		return nil, connectErr(addErr)
	}
	return connect.NewResponse(&todosv1.CreateURLPatternResponse{}), nil
}

func (h *settingsConnectHandler) DeleteURLPattern(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteURLPatternRequest],
) (*connect.Response[todosv1.DeleteURLPatternResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Settings.DeleteURLPattern(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteURLPatternResponse{}), nil
}
