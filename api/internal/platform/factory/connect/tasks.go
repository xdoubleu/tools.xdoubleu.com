package connect

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/factory/present"
)

// Deps is what the Connect handlers and the MCP tools share.
type Deps struct {
	Services *services.Services
	Logger   *slog.Logger
	// HeartbeatHold is how long a runner's Heartbeat or ClaimJob is held open.
	HeartbeatHold time.Duration
	// StepUp checks the TOTP code that creating a runner needs.
	StepUp StepUp
	// Chats pushes new chat events to the sockets watching them; nil pushes none.
	Chats ChatPublisher
}

// Present converts task data for the proto DTOs.
func (f *Deps) Present() present.Presenter {
	return present.Presenter{Labels: f.Services.Jobs}
}

type taskConnectHandler struct {
	app *Deps
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
		Via:     "",
	}, nil
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
	case factoryv1.TaskStatus_TASK_STATUS_TESTING:
		return models.StatusTesting
	case factoryv1.TaskStatus_TASK_STATUS_UNSPECIFIED:
		return ""
	}
	return ""
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

func (h *taskConnectHandler) ListTasks(
	ctx context.Context,
	_ *factoryv1.ListTasksRequest,
) (*factoryv1.ListTasksResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	board, err := h.app.Services.Board.ListTasks(ctx, actor, models.TaskFilter{
		Query: "", Status: "", RepoID: nil, Label: "",
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	limited := ""
	if board.LimitedUntil != nil {
		limited = board.LimitedUntil.UTC().Format(time.RFC3339)
	}
	return &factoryv1.ListTasksResponse{
		Tasks: h.app.Present().Tasks(ctx, board.Tasks), PickupPaused: board.PickupPaused,
		ClaudeLimitedUntil: limited, QueueStalled: board.QueueStalled,
	}, nil
}

func (h *taskConnectHandler) GetTask(
	ctx context.Context,
	req *factoryv1.GetTaskRequest,
) (*factoryv1.GetTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	detail, err := h.app.Services.Board.GetTask(ctx, actor, req.Id)
	if err != nil {
		return nil, present.MapError(err)
	}
	return h.app.Present().Detail(ctx, detail), nil
}

func (h *taskConnectHandler) CreateTask(
	ctx context.Context,
	req *factoryv1.CreateTaskRequest,
) (*factoryv1.CreateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := present.ParseID(req.RepoId)
	if err != nil {
		return nil, err
	}
	task, err := h.app.Services.Board.CreateTask(ctx, actor, repoID, services.TaskInput{
		Title:    req.Title,
		Body:     req.Body,
		Priority: priorityFromProto(req.Priority),
		Labels:   req.Labels,
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: h.app.Present().Task(ctx, task)}, nil
}

func (h *taskConnectHandler) UpdateTask(
	ctx context.Context,
	req *factoryv1.UpdateTaskRequest,
) (*factoryv1.UpdateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	priority := priorityFromProto(req.Priority)
	task, err := h.app.Services.Board.UpdateTask(ctx, actor, req.Id, services.TaskPatch{
		Title:        &req.Title,
		Body:         &req.Body,
		Priority:     &priority,
		Labels:       &req.Labels,
		AddLabels:    nil,
		RemoveLabels: nil,
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.UpdateTaskResponse{Task: h.app.Present().Task(ctx, task)}, nil
}

func (h *taskConnectHandler) MoveTask(
	ctx context.Context,
	req *factoryv1.MoveTaskRequest,
) (*factoryv1.MoveTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	task, err := h.app.Services.Board.MoveTask(
		ctx, actor, req.Id, statusFromProto(req.Status))
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: h.app.Present().Task(ctx, task)}, nil
}

func (h *taskConnectHandler) AddComment(
	ctx context.Context,
	req *factoryv1.AddCommentRequest,
) (*factoryv1.AddCommentResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	comment, err := h.app.Services.Board.AddComment(ctx, actor, req.TaskId, req.Body)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.AddCommentResponse{
		Comment: h.app.Present().Comment(ctx, comment),
	}, nil
}

// taskID resolves a task reference from a request.
func (f *Deps) taskID(ctx context.Context, ref string) (uuid.UUID, error) {
	id, err := f.Services.Tasks.Resolve(ctx, ref)
	if err != nil {
		return uuid.Nil, present.MapError(err)
	}
	return id, nil
}
