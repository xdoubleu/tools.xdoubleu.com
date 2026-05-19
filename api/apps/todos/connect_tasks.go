package todos

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

func (h *taskConnectHandler) ListTasks(
	ctx context.Context,
	req *connect.Request[todosv1.ListTasksRequest],
) (*connect.Response[todosv1.ListTasksResponse], error) {
	userID := h.userID(ctx)

	var sectionID *uuid.UUID
	if req.Msg.SectionId != "" {
		if id, err := uuid.Parse(req.Msg.SectionId); err == nil {
			sectionID = &id
		}
	}

	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if id, err := uuid.Parse(req.Msg.WorkspaceId); err == nil {
			workspaceID = &id
		}
	}

	var (
		tasks []models.Task
		err   error
	)

	switch req.Msg.Status {
	case "done":
		tasks, err = h.app.services.Tasks.List(
			ctx,
			userID,
			models.StatusDone,
			workspaceID,
		)
	case "archived":
		tasks, err = h.app.services.Tasks.Search(ctx, userID, "", workspaceID)
	default:
		tasks, err = h.app.services.Tasks.ListOpen(ctx, userID, sectionID, workspaceID)
	}
	if err != nil {
		return nil, connectErr(err)
	}

	return connect.NewResponse(&todosv1.ListTasksResponse{
		Tasks: protoTasks(tasks),
	}), nil
}

func (h *taskConnectHandler) GetTask(
	ctx context.Context,
	req *connect.Request[todosv1.GetTaskRequest],
) (*connect.Response[todosv1.GetTaskResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	task, err := h.app.services.Tasks.Get(ctx, id, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.GetTaskResponse{Task: protoTask(*task)}), nil
}

func (h *taskConnectHandler) CreateTask(
	ctx context.Context,
	req *connect.Request[todosv1.CreateTaskRequest],
) (*connect.Response[todosv1.CreateTaskResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	dto := dtos.SaveTaskDto{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Label:       req.Msg.Label,
		DueDate:     req.Msg.DueDate,
		Deadline:    req.Msg.Deadline,
		SectionID:   req.Msg.SectionId,
		Priority:    int(req.Msg.Priority),
		RecurDays:   int(req.Msg.RecurDays),
		Recur:       req.Msg.RecurRule,
		LinkURLs:    nil,
		LinkLabels:  nil,
	}
	task, err := h.app.services.Tasks.Create(
		ctx, userID, wsCtx.Settings.ActiveWorkspaceID, dto,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.CreateTaskResponse{Task: protoTask(*task)}), nil
}

func (h *taskConnectHandler) UpdateTask(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateTaskRequest],
) (*connect.Response[todosv1.UpdateTaskResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	dto := dtos.SaveTaskDto{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Label:       req.Msg.Label,
		DueDate:     req.Msg.DueDate,
		Deadline:    req.Msg.Deadline,
		SectionID:   req.Msg.SectionId,
		Priority:    int(req.Msg.Priority),
		RecurDays:   int(req.Msg.RecurDays),
		Recur:       req.Msg.RecurRule,
		LinkURLs:    nil,
		LinkLabels:  nil,
	}
	if err = h.app.services.Tasks.Update(
		ctx, id, userID, wsCtx.Settings.ActiveWorkspaceID, dto,
	); err != nil {
		return nil, connectErr(err)
	}
	task, err := h.app.services.Tasks.Get(ctx, id, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdateTaskResponse{Task: protoTask(*task)}), nil
}

func (h *taskConnectHandler) CompleteTask(
	ctx context.Context,
	req *connect.Request[todosv1.CompleteTaskRequest],
) (*connect.Response[todosv1.CompleteTaskResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Tasks.Complete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.CompleteTaskResponse{}), nil
}

func (h *taskConnectHandler) ReopenTask(
	ctx context.Context,
	req *connect.Request[todosv1.ReopenTaskRequest],
) (*connect.Response[todosv1.ReopenTaskResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Tasks.Reopen(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.ReopenTaskResponse{}), nil
}

func (h *taskConnectHandler) DeleteTask(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteTaskRequest],
) (*connect.Response[todosv1.DeleteTaskResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Tasks.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteTaskResponse{}), nil
}

func (h *taskConnectHandler) ReorderTasks(
	ctx context.Context,
	req *connect.Request[todosv1.ReorderTasksRequest],
) (*connect.Response[todosv1.ReorderTasksResponse], error) {
	userID := h.userID(ctx)
	ids := make([]uuid.UUID, 0, len(req.Msg.Ids))
	for _, s := range req.Msg.Ids {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	if err := h.app.services.Tasks.Reorder(ctx, userID, ids); err != nil {
		return nil, connectErr(err)
	}
	resp := &todosv1.ReorderTasksResponse{}
	return connect.NewResponse(resp), nil
}

func (h *taskConnectHandler) SearchTasks(
	ctx context.Context,
	req *connect.Request[todosv1.SearchTasksRequest],
) (*connect.Response[todosv1.SearchTasksResponse], error) {
	userID := h.userID(ctx)
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if id, err := uuid.Parse(req.Msg.WorkspaceId); err == nil {
			workspaceID = &id
		}
	}
	tasks, err := h.app.services.Tasks.SearchAll(
		ctx,
		userID,
		req.Msg.Query,
		workspaceID,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	var open, done, archived []models.Task
	for _, t := range tasks {
		switch t.Status {
		case models.StatusDone:
			done = append(done, t)
		case models.StatusArchived:
			archived = append(archived, t)
		default:
			open = append(open, t)
		}
	}
	return connect.NewResponse(&todosv1.SearchTasksResponse{
		Open:     protoTasks(open),
		Done:     protoTasks(done),
		Archived: protoTasks(archived),
	}), nil
}

func (h *taskConnectHandler) QuickAddTask(
	ctx context.Context,
	req *connect.Request[todosv1.QuickAddTaskRequest],
) (*connect.Response[todosv1.QuickAddTaskResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	task, err := h.app.services.Tasks.QuickAdd(
		ctx, userID, req.Msg.Input, req.Msg.Description,
		wsCtx.Settings.ActiveWorkspaceID, req.Msg.SectionId,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(
		&todosv1.QuickAddTaskResponse{Task: protoTask(*task)},
	), nil
}

func (h *taskConnectHandler) QuickUpdateTask(
	ctx context.Context,
	req *connect.Request[todosv1.QuickUpdateTaskRequest],
) (*connect.Response[todosv1.QuickUpdateTaskResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	_, err = h.app.services.Tasks.QuickUpdate(
		ctx, id, userID, wsCtx.Settings.ActiveWorkspaceID,
		req.Msg.Input, req.Msg.Description,
	)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.QuickUpdateTaskResponse{}), nil
}

func (h *taskConnectHandler) MoveTaskSection(
	ctx context.Context,
	req *connect.Request[todosv1.MoveTaskSectionRequest],
) (*connect.Response[todosv1.MoveTaskSectionResponse], error) {
	userID := h.userID(ctx)
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	var sectionID *uuid.UUID
	if req.Msg.SectionId != "" {
		if sid, parseErr := uuid.Parse(req.Msg.SectionId); parseErr == nil {
			sectionID = &sid
		}
	}
	if moveErr := h.app.services.Tasks.MoveSection(
		ctx, taskID, userID, sectionID,
	); moveErr != nil {
		return nil, connectErr(moveErr)
	}
	return connect.NewResponse(&todosv1.MoveTaskSectionResponse{}), nil
}
