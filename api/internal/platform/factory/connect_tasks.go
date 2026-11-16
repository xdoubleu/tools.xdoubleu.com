package factory

import (
	"context"
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
		Via:     "",
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
func (f *Factory) actorLabels(
	ctx context.Context,
	actors ...string,
) map[string]string {
	return f.services.Jobs.ActorLabels(ctx, actors)
}

// protoTasks converts tasks with their agent creators' owners shown.
func (f *Factory) protoTasks(
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

// protoTaskOf converts one task with its agent creator's owner shown.
func (f *Factory) protoTaskOf(ctx context.Context, t *models.Task) *factoryv1.Task {
	return protoTask(t, f.actorLabels(ctx, t.CreatedBy))
}

// protoCommentOf converts one comment with its agent author's owner shown.
func (f *Factory) protoCommentOf(
	ctx context.Context,
	c *models.Comment,
) *factoryv1.Comment {
	return protoComment(c, f.actorLabels(ctx, c.Author))
}

// protoComments converts comments with their agent authors' owners shown.
func (f *Factory) protoComments(
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
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	// The board is polled while a runner may be gone, so this is where a job that
	// stopped checking in gets noticed and its task parked in Needs you.
	h.app.services.Jobs.SweepLost(ctx)
	tasks, err := h.app.services.Tasks.List(ctx, "")
	if err != nil {
		return nil, mapError(err)
	}
	h.app.services.Jobs.FillDeploy(ctx, tasks)
	out := h.app.protoTasks(ctx, tasks)
	paused, err := h.app.services.Jobs.PickupPaused(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	// The limit notice is a hint: if it cannot be read, the board still loads.
	until, _ := h.app.services.Jobs.ActiveLimit(ctx)
	limited := ""
	if until != nil {
		limited = until.UTC().Format(time.RFC3339)
	}
	// Also a hint, like the limit notice.
	stalled, _ := h.app.services.Jobs.Stalled(ctx)
	return &factoryv1.ListTasksResponse{
		Tasks: out, PickupPaused: paused, ClaudeLimitedUntil: limited,
		QueueStalled: stalled,
	}, nil
}

func (h *taskConnectHandler) GetTask(
	ctx context.Context,
	req *factoryv1.GetTaskRequest,
) (*factoryv1.GetTaskResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	// The sheet polls this while a job runs, so a dead runner's job must be
	// marked lost here too.
	h.app.services.Jobs.SweepLost(ctx)
	task, comments, err := h.app.services.Tasks.Get(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	out := h.app.protoComments(ctx, comments)
	filled := []models.Task{*task}
	h.app.services.Jobs.FillDeploy(ctx, filled)
	task = &filled[0]
	jobs, err := h.app.services.Jobs.ByTask(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	jobOut := make([]*factoryv1.Job, len(jobs))
	for i := range jobs {
		jobOut[i] = protoJob(&jobs[i])
	}
	pulls, err := h.app.services.Jobs.PullRequestsOf(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	pullOut := make([]*factoryv1.PullRequest, len(pulls))
	for i := range pulls {
		pullOut[i] = &factoryv1.PullRequest{
			Url: pulls[i].URL, State: pulls[i].State, Ci: pulls[i].CI,
			Review: pulls[i].Review, Mergeable: pulls[i].Mergeable,
		}
	}
	return &factoryv1.GetTaskResponse{
		Task: h.app.protoTaskOf(ctx, task), Comments: out, Jobs: jobOut,
		PullRequests: pullOut,
	}, nil
}

func (h *taskConnectHandler) CreateTask(
	ctx context.Context,
	req *factoryv1.CreateTaskRequest,
) (*factoryv1.CreateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := parseID(req.RepoId)
	if err != nil {
		return nil, err
	}
	task, err := h.app.services.Tasks.Create(ctx, actor, repoID, services.TaskInput{
		Title:    req.Title,
		Body:     req.Body,
		Priority: priorityFromProto(req.Priority),
		Labels:   req.Labels,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: h.app.protoTaskOf(ctx, task)}, nil
}

func (h *taskConnectHandler) UpdateTask(
	ctx context.Context,
	req *factoryv1.UpdateTaskRequest,
) (*factoryv1.UpdateTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.Id)
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
	return &factoryv1.UpdateTaskResponse{Task: h.app.protoTaskOf(ctx, task)}, nil
}

func (h *taskConnectHandler) MoveTask(
	ctx context.Context,
	req *factoryv1.MoveTaskRequest,
) (*factoryv1.MoveTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	task, err := h.app.services.Tasks.Move(ctx, actor, id, statusFromProto(req.Status))
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: h.app.protoTaskOf(ctx, task)}, nil
}

func (h *taskConnectHandler) AddComment(
	ctx context.Context,
	req *factoryv1.AddCommentRequest,
) (*factoryv1.AddCommentResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.TaskId)
	if err != nil {
		return nil, err
	}
	comment, err := h.app.services.Tasks.Comment(ctx, actor, id, req.Body)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.AddCommentResponse{Comment: h.app.protoCommentOf(ctx, comment)}, nil
}

// taskID resolves a task reference from a request.
func (f *Factory) taskID(ctx context.Context, ref string) (uuid.UUID, error) {
	id, err := f.services.Tasks.Resolve(ctx, ref)
	if err != nil {
		return uuid.Nil, mapError(err)
	}
	return id, nil
}
