// Package present maps factory models to their proto DTOs and service errors
// to Connect errors, for the Connect handlers and the MCP tools alike.
package present

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

// ActorLabeler names the owner of agent actors.
type ActorLabeler interface {
	ActorLabels(ctx context.Context, actors []string) map[string]string
}

// Presenter converts task data, which shows agents' owners.
type Presenter struct {
	Labels ActorLabeler
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

// labelled is the actor with its owner when it is an agent, else as it is.
func labelled(labels map[string]string, actor string) string {
	if label, ok := labels[actor]; ok {
		return label
	}
	return actor
}

// actorLabels names the owner of every agent among the actors, in one lookup.
func (f Presenter) actorLabels(
	ctx context.Context,
	actors ...string,
) map[string]string {
	return f.Labels.ActorLabels(ctx, actors)
}

// Tasks converts tasks with their agent creators' owners shown.
func (f Presenter) Tasks(
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

// Task converts one task with its agent creator's owner shown.
func (f Presenter) Task(ctx context.Context, t *models.Task) *factoryv1.Task {
	return protoTask(t, f.actorLabels(ctx, t.CreatedBy))
}

// Comment converts one comment with its agent author's owner shown.
func (f Presenter) Comment(
	ctx context.Context,
	c *models.Comment,
) *factoryv1.Comment {
	return protoComment(c, f.actorLabels(ctx, c.Author))
}

// Comments converts comments with their agent authors' owners shown.
func (f Presenter) Comments(
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

func jobStatusToProto(s models.JobStatus) factoryv1.JobStatus {
	switch s {
	case models.JobQueued:
		return factoryv1.JobStatus_JOB_STATUS_QUEUED
	case models.JobRunning:
		return factoryv1.JobStatus_JOB_STATUS_RUNNING
	case models.JobSucceeded:
		return factoryv1.JobStatus_JOB_STATUS_SUCCEEDED
	case models.JobFailed:
		return factoryv1.JobStatus_JOB_STATUS_FAILED
	case models.JobCancelled:
		return factoryv1.JobStatus_JOB_STATUS_CANCELLED
	case models.JobLost:
		return factoryv1.JobStatus_JOB_STATUS_LOST
	}
	return factoryv1.JobStatus_JOB_STATUS_UNSPECIFIED
}

// ProtoJob is a job as the board shows it.
func ProtoJob(j *models.Job) *factoryv1.Job {
	limitResetsAt := ""
	if j.LimitResetsAt != nil {
		limitResetsAt = j.LimitResetsAt.UTC().Format(time.RFC3339)
	}
	taskID := ""
	if !j.IsRoutine() {
		taskID = models.TaskRef(j.TaskNumber)
	}
	return &factoryv1.Job{
		Id:        j.ID.String(),
		TaskId:    taskID,
		Routine:   j.Routine,
		Status:    jobStatusToProto(j.Status),
		Branch:    j.Branch,
		CreatedAt: j.CreatedAt.Format(time.RFC3339),
		PrUrl:     j.PRURL,
		PrState:   j.PRState,
		PrCi:      j.PRCI,
		PrReview:  j.PRReview,
		Harness:   HarnessToProto(j.Harness),
		Model:     j.Model,

		Limited:       j.Limited,
		LimitResetsAt: limitResetsAt,
		Outcome:       truncateOutcome(j.Outcome),
	}
}

// ProtoJobs converts jobs with ProtoJob.
func ProtoJobs(jobs []models.Job) []*factoryv1.Job {
	out := make([]*factoryv1.Job, len(jobs))
	for i := range jobs {
		out[i] = ProtoJob(&jobs[i])
	}
	return out
}

// ProtoJobEvents converts a job log page.
func ProtoJobEvents(events []models.JobEvent) []*factoryv1.JobEvent {
	out := make([]*factoryv1.JobEvent, len(events))
	for i := range events {
		out[i] = &factoryv1.JobEvent{
			Seq:       int32(events[i].Seq), //nolint:gosec // capped per job
			Kind:      events[i].Kind,
			Payload:   events[i].Payload,
			CreatedAt: events[i].CreatedAt.Format(time.RFC3339),
		}
	}
	return out
}

func HarnessToProto(h models.Harness) factoryv1.Harness {
	switch h {
	case models.HarnessClaudeCode:
		return factoryv1.Harness_HARNESS_CLAUDE_CODE
	case models.HarnessOpenCode:
		return factoryv1.Harness_HARNESS_OPENCODE
	case models.HarnessInvalid:
		return factoryv1.Harness_HARNESS_UNSPECIFIED
	}
	return factoryv1.Harness_HARNESS_UNSPECIFIED
}

// ProtoRoutine is a routine with its newest job.
func ProtoRoutine(r *models.Routine) *factoryv1.Routine {
	out := &factoryv1.Routine{
		Name:            r.Name,
		RepoId:          r.RepoID.String(),
		Enabled:         r.Enabled,
		IntervalMinutes: int32(r.IntervalMinutes), //nolint:gosec // capped to a month
		LastJob:         nil,
	}
	if r.LastJob != nil {
		out.LastJob = ProtoJob(r.LastJob)
	}
	return out
}

// maxRoutineOutcome caps a job's outcome as the board shows it.
const maxRoutineOutcome = 2000

func truncateOutcome(s string) string {
	if len(s) <= maxRoutineOutcome {
		return s
	}
	return strings.ToValidUTF8(s[:maxRoutineOutcome], "") + "…"
}

// Detail is a task with its comments, jobs and pull requests.
func (f Presenter) Detail(
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
		Task: f.Task(ctx, d.Task), Comments: f.Comments(ctx, d.Comments),
		Jobs: ProtoJobs(d.Jobs), PullRequests: pulls,
	}
}

// AgentRepo is a repo as agents see it: the setup commands and instructions
// are for the sandbox.
func AgentRepo(r *models.Repo) *factoryv1.Repo {
	return &factoryv1.Repo{
		Id:            r.ID.String(),
		Owner:         r.Owner,
		Name:          r.Name,
		DefaultBranch: r.DefaultBranch,
	}
}
