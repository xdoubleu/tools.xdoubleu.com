package connect

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

// Deps is what the Connect handlers and the MCP tools share.
type Deps struct {
	Services *services.Services
	Logger   *slog.Logger
	// HeartbeatHold is how long a runner's Heartbeat or ClaimJob is held open.
	HeartbeatHold time.Duration
	// StepUp checks the TOTP code that creating a runner needs.
	StepUp StepUp
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

// MapError turns a service error into a Connect error.
func MapError(err error) error {
	return connecttools.MapError(err,
		connecttools.Code(services.ErrInvalid, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrForbidden, connect.CodePermissionDenied),
		connecttools.Code(services.ErrConflict, connect.CodeAlreadyExists),
	)
}

// ParseID reads a UUID from a request field.
func ParseID(id string) (uuid.UUID, error) {
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
	case models.StatusTesting:
		return factoryv1.TaskStatus_TASK_STATUS_TESTING
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
	case factoryv1.TaskStatus_TASK_STATUS_TESTING:
		return models.StatusTesting
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

// labelled is the actor with its owner when it is an agent, else as it is.
func labelled(labels map[string]string, actor string) string {
	if label, ok := labels[actor]; ok {
		return label
	}
	return actor
}

// actorLabels names the owner of every agent among the actors, in one lookup.
func (f *Deps) actorLabels(
	ctx context.Context,
	actors ...string,
) map[string]string {
	return f.Services.Jobs.ActorLabels(ctx, actors)
}

// ProtoTasks converts tasks with their agent creators' owners shown.
func (f *Deps) ProtoTasks(
	ctx context.Context,
	tasks []models.Task,
) []*factoryv1.Task {
	actors := make([]string, len(tasks))
	for i := range tasks {
		actors[i] = tasks[i].CreatedBy
	}
	labels := f.actorLabels(ctx, actors...)
	out := make([]*factoryv1.Task, len(tasks))
	for i := range tasks {
		out[i] = protoTask(&tasks[i], labels)
	}
	return out
}

// ProtoTaskOf converts one task with its agent creator's owner shown.
func (f *Deps) ProtoTaskOf(ctx context.Context, t *models.Task) *factoryv1.Task {
	return protoTask(t, f.actorLabels(ctx, t.CreatedBy))
}

// ProtoCommentOf converts one comment with its agent author's owner shown.
func (f *Deps) ProtoCommentOf(
	ctx context.Context,
	c *models.Comment,
) *factoryv1.Comment {
	return protoComment(c, f.actorLabels(ctx, c.Author))
}

// ProtoComments converts comments with their agent authors' owners shown.
func (f *Deps) ProtoComments(
	ctx context.Context,
	comments []models.Comment,
) []*factoryv1.Comment {
	actors := make([]string, len(comments))
	for i := range comments {
		actors[i] = comments[i].Author
	}
	labels := f.actorLabels(ctx, actors...)
	out := make([]*factoryv1.Comment, len(comments))
	for i := range comments {
		out[i] = protoComment(&comments[i], labels)
	}
	return out
}

func protoTask(t *models.Task, labels map[string]string) *factoryv1.Task {
	return &factoryv1.Task{
		Id:                models.TaskRef(t.Number),
		RepoId:            t.RepoID.String(),
		Title:             t.Title,
		Body:              t.Body,
		Status:            statusToProto(t.Status),
		Priority:          priorityToProto(t.Priority),
		Labels:            t.Labels,
		CreatedBy:         labelled(labels, t.CreatedBy),
		CreatedAt:         t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         t.UpdatedAt.Format(time.RFC3339),
		CommentCount:      int32(t.CommentCount), //nolint:gosec // a count, far below 2^31
		PrUrl:             t.PRURL,
		PrState:           t.PRState,
		PrCi:              t.PRCI,
		PrReview:          t.PRReview,
		PrMergeable:       t.PRMergeable,
		PickupApproved:    t.PickupApproved,
		PickupBlockReason: t.PickupBlocked,
		DeployState:       string(t.Deploy),
		DeployReason:      t.DeployReason,
	}
}

func protoComment(
	c *models.Comment,
	labels map[string]string,
) *factoryv1.Comment {
	return &factoryv1.Comment{
		Id:        c.ID.String(),
		Author:    labelled(labels, c.Author),
		Body:      c.Body,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
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
		return nil, MapError(err)
	}
	limited := ""
	if board.LimitedUntil != nil {
		limited = board.LimitedUntil.UTC().Format(time.RFC3339)
	}
	return &factoryv1.ListTasksResponse{
		Tasks: h.app.ProtoTasks(ctx, board.Tasks), PickupPaused: board.PickupPaused,
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
		return nil, MapError(err)
	}
	return h.app.ProtoDetail(ctx, detail), nil
}

// ProtoDetail is a task with its comments, jobs and pull requests.
func (f *Deps) ProtoDetail(
	ctx context.Context,
	d *services.TaskDetail,
) *factoryv1.GetTaskResponse {
	pulls := make([]*factoryv1.PullRequest, len(d.Pulls))
	for i := range d.Pulls {
		pulls[i] = &factoryv1.PullRequest{
			Url: d.Pulls[i].URL, State: d.Pulls[i].State, Ci: d.Pulls[i].CI,
			Review: d.Pulls[i].Review, Mergeable: d.Pulls[i].Mergeable,
		}
	}
	return &factoryv1.GetTaskResponse{
		Task: f.ProtoTaskOf(ctx, d.Task), Comments: f.ProtoComments(ctx, d.Comments),
		Jobs: ProtoJobs(d.Jobs), PullRequests: pulls,
	}
}

func (h *taskConnectHandler) CreateTask(
	ctx context.Context,
	req *factoryv1.CreateTaskRequest,
) (*factoryv1.CreateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := ParseID(req.RepoId)
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
		return nil, MapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: h.app.ProtoTaskOf(ctx, task)}, nil
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
		return nil, MapError(err)
	}
	return &factoryv1.UpdateTaskResponse{Task: h.app.ProtoTaskOf(ctx, task)}, nil
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
		return nil, MapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: h.app.ProtoTaskOf(ctx, task)}, nil
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
		return nil, MapError(err)
	}
	return &factoryv1.AddCommentResponse{Comment: h.app.ProtoCommentOf(ctx, comment)}, nil
}

// taskID resolves a task reference from a request.
func (f *Deps) taskID(ctx context.Context, ref string) (uuid.UUID, error) {
	id, err := f.Services.Tasks.Resolve(ctx, ref)
	if err != nil {
		return uuid.Nil, MapError(err)
	}
	return id, nil
}
