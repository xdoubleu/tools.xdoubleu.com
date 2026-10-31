package factory

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type taskConnectHandler struct {
	app *Factory
}

var _ factoryv1connect.TaskServiceHandler = (*taskConnectHandler)(nil)

// actorFrom derives the actor from the request: the factory is admin-only, so
// anyone else is denied, and the auth channel decides whether the actor is a
// browser session. A request with no recorded channel is not a browser.
func actorFrom(ctx context.Context) (models.Actor, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return models.Actor{}, connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	if user.Role != sharedmodels.RoleAdmin {
		return models.Actor{}, connect.NewError(
			connect.CodePermissionDenied, "admin access required",
		)
	}
	channel := contexttools.GetValue[string](ctx, constants.AuthChannelContextKey)
	return models.Actor{
		Kind:    models.ActorHuman,
		ID:      user.ID,
		Browser: channel != nil && *channel == constants.ChannelBrowser,
		Admin:   user.Role == sharedmodels.RoleAdmin,
	}, nil
}

func mapError(err error) error {
	return connecttools.MapError(err)
}

func parseID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, "invalid id")
	}
	return parsed, nil
}

func statusToProto(s models.Status) factoryv1.TaskStatus {
	switch s {
	case models.StatusBacklog:
		return factoryv1.TaskStatus_TASK_STATUS_BACKLOG
	case models.StatusReady:
		return factoryv1.TaskStatus_TASK_STATUS_READY
	case models.StatusInProgress:
		return factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS
	case models.StatusInReview:
		return factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW
	case models.StatusNeedsYou:
		return factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU
	case models.StatusDone:
		return factoryv1.TaskStatus_TASK_STATUS_DONE
	}
	return factoryv1.TaskStatus_TASK_STATUS_UNSPECIFIED
}

// statusFromProto maps an unknown or unspecified status to "", which the
// service rejects.
func statusFromProto(s factoryv1.TaskStatus) models.Status {
	switch s {
	case factoryv1.TaskStatus_TASK_STATUS_BACKLOG:
		return models.StatusBacklog
	case factoryv1.TaskStatus_TASK_STATUS_READY:
		return models.StatusReady
	case factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return models.StatusInProgress
	case factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW:
		return models.StatusInReview
	case factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU:
		return models.StatusNeedsYou
	case factoryv1.TaskStatus_TASK_STATUS_DONE:
		return models.StatusDone
	case factoryv1.TaskStatus_TASK_STATUS_UNSPECIFIED:
		return ""
	}
	return ""
}

func priorityToProto(p models.Priority) factoryv1.TaskPriority {
	switch p {
	case models.PriorityP0:
		return factoryv1.TaskPriority_TASK_PRIORITY_P0
	case models.PriorityP1:
		return factoryv1.TaskPriority_TASK_PRIORITY_P1
	case models.PriorityP2:
		return factoryv1.TaskPriority_TASK_PRIORITY_P2
	case models.PriorityInvalid:
		return factoryv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED
	}
	return factoryv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED
}

// priorityFromProto maps unspecified to "", which the service defaults on
// create, and an unknown value to one the service rejects.
func priorityFromProto(p factoryv1.TaskPriority) models.Priority {
	switch p {
	case factoryv1.TaskPriority_TASK_PRIORITY_P0:
		return models.PriorityP0
	case factoryv1.TaskPriority_TASK_PRIORITY_P1:
		return models.PriorityP1
	case factoryv1.TaskPriority_TASK_PRIORITY_P2:
		return models.PriorityP2
	case factoryv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED:
		return ""
	}
	return models.PriorityInvalid
}

func protoTask(t *models.Task) *factoryv1.Task {
	return &factoryv1.Task{
		Id:           t.ID.String(),
		Title:        t.Title,
		Body:         t.Body,
		Status:       statusToProto(t.Status),
		Priority:     priorityToProto(t.Priority),
		Labels:       t.Labels,
		CreatedBy:    t.CreatedBy,
		CreatedAt:    t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    t.UpdatedAt.Format(time.RFC3339),
		CommentCount: int32(t.CommentCount), //nolint:gosec // a count, far below 2^31
	}
}

func protoComment(c *models.Comment) *factoryv1.Comment {
	return &factoryv1.Comment{
		Id:        c.ID.String(),
		Author:    c.Author,
		Body:      c.Body,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
}

func (h *taskConnectHandler) ListTasks(
	ctx context.Context,
	_ *factoryv1.ListTasksRequest,
) (*factoryv1.ListTasksResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	tasks, err := h.app.services.Tasks.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*factoryv1.Task, len(tasks))
	for i := range tasks {
		out[i] = protoTask(&tasks[i])
	}
	return &factoryv1.ListTasksResponse{Tasks: out}, nil
}

func (h *taskConnectHandler) GetTask(
	ctx context.Context,
	req *factoryv1.GetTaskRequest,
) (*factoryv1.GetTaskResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}
	task, comments, err := h.app.services.Tasks.Get(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*factoryv1.Comment, len(comments))
	for i := range comments {
		out[i] = protoComment(&comments[i])
	}
	return &factoryv1.GetTaskResponse{Task: protoTask(task), Comments: out}, nil
}

func (h *taskConnectHandler) CreateTask(
	ctx context.Context,
	req *factoryv1.CreateTaskRequest,
) (*factoryv1.CreateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	task, err := h.app.services.Tasks.Create(ctx, actor, services.TaskInput{
		Title:    req.Title,
		Body:     req.Body,
		Priority: priorityFromProto(req.Priority),
		Labels:   req.Labels,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: protoTask(task)}, nil
}

func (h *taskConnectHandler) UpdateTask(
	ctx context.Context,
	req *factoryv1.UpdateTaskRequest,
) (*factoryv1.UpdateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}
	task, err := h.app.services.Tasks.Update(ctx, actor, id, services.TaskInput{
		Title:    req.Title,
		Body:     req.Body,
		Priority: priorityFromProto(req.Priority),
		Labels:   req.Labels,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.UpdateTaskResponse{Task: protoTask(task)}, nil
}

func (h *taskConnectHandler) MoveTask(
	ctx context.Context,
	req *factoryv1.MoveTaskRequest,
) (*factoryv1.MoveTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}
	task, err := h.app.services.Tasks.Move(ctx, actor, id, statusFromProto(req.Status))
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: protoTask(task)}, nil
}

func (h *taskConnectHandler) AddComment(
	ctx context.Context,
	req *factoryv1.AddCommentRequest,
) (*factoryv1.AddCommentResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.TaskId)
	if err != nil {
		return nil, err
	}
	comment, err := h.app.services.Tasks.Comment(ctx, actor, id, req.Body)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.AddCommentResponse{Comment: protoComment(comment)}, nil
}
