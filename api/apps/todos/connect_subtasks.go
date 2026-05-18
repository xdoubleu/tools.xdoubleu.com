package todos

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

func (h *subtaskConnectHandler) AddSubtask(
	ctx context.Context,
	req *connect.Request[todosv1.AddSubtaskRequest],
) (*connect.Response[todosv1.AddSubtaskResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var parentID *uuid.UUID
	if req.Msg.ParentSubtaskId != "" {
		if pid, parseErr := uuid.Parse(req.Msg.ParentSubtaskId); parseErr == nil {
			parentID = &pid
		}
	}
	subtask, err := h.app.services.Tasks.AddSubtask(
		ctx, taskID, userID, wsCtx.Settings.ActiveWorkspaceID,
		req.Msg.Input, req.Msg.Description, parentID,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.AddSubtaskResponse{
		Subtask: protoSubtask(*subtask),
	}), nil
}

func (h *subtaskConnectHandler) ToggleSubtask(
	ctx context.Context,
	req *connect.Request[todosv1.ToggleSubtaskRequest],
) (*connect.Response[todosv1.ToggleSubtaskResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	sid, err := uuid.Parse(req.Msg.SubtaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Tasks.ToggleSubtask(ctx, sid, taskID, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.ToggleSubtaskResponse{
		Subtask: nil,
	}), nil
}

func (h *subtaskConnectHandler) DeleteSubtask(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteSubtaskRequest],
) (*connect.Response[todosv1.DeleteSubtaskResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	sid, err := uuid.Parse(req.Msg.SubtaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Tasks.DeleteSubtask(ctx, sid, taskID, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteSubtaskResponse{}), nil
}

func (h *subtaskConnectHandler) ReorderSubtasks(
	ctx context.Context,
	req *connect.Request[todosv1.ReorderSubtasksRequest],
) (*connect.Response[todosv1.ReorderSubtasksResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	ids := make([]uuid.UUID, 0, len(req.Msg.Ids))
	for _, s := range req.Msg.Ids {
		if id, parseErr := uuid.Parse(s); parseErr == nil {
			ids = append(ids, id)
		}
	}
	var parentID *uuid.UUID
	if req.Msg.ParentSubtaskId != "" {
		if pid, parseErr := uuid.Parse(req.Msg.ParentSubtaskId); parseErr == nil {
			parentID = &pid
		}
	}
	if reorderErr := h.app.services.Tasks.ReorderSubtasks(
		ctx, taskID, userID, ids, parentID,
	); reorderErr != nil {
		return nil, connectErr(reorderErr)
	}
	return connect.NewResponse(&todosv1.ReorderSubtasksResponse{}), nil
}

func (h *subtaskConnectHandler) UpdateSubtask(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateSubtaskRequest],
) (*connect.Response[todosv1.UpdateSubtaskResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	sid, err := uuid.Parse(req.Msg.SubtaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	dto := dtos.UpdateSubtaskDto{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Priority:    int(req.Msg.Priority),
		Label:       req.Msg.Label,
		DueDate:     req.Msg.DueDate,
		Deadline:    req.Msg.Deadline,
	}
	subtask, err := h.app.services.Tasks.UpdateSubtask(
		ctx, sid, taskID, userID, wsCtx.Settings.ActiveWorkspaceID, dto,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdateSubtaskResponse{
		Subtask: protoSubtask(*subtask),
	}), nil
}
