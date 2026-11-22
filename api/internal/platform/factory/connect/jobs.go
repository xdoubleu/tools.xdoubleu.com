package connect

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/factory/present"
)

func (h *taskConnectHandler) RunTask(
	ctx context.Context,
	req *factoryv1.RunTaskRequest,
) (*factoryv1.RunTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.GetTaskId())
	if err != nil {
		return nil, err
	}
	job, err := h.app.Services.Jobs.Run(
		ctx,
		actor,
		id,
	)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.RunTaskResponse{Job: present.ProtoJob(job)}, nil
}

func (h *taskConnectHandler) SetPickupPaused(
	ctx context.Context,
	req *factoryv1.SetPickupPausedRequest,
) (*factoryv1.SetPickupPausedResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err = h.app.Services.Jobs.SetPickupPaused(ctx, actor, req.GetPaused()); err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.SetPickupPausedResponse{Paused: req.GetPaused()}, nil
}

func (h *taskConnectHandler) StopJob(
	ctx context.Context,
	req *factoryv1.StopJobRequest,
) (*factoryv1.StopJobResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.app.taskID(ctx, req.GetTaskId())
	if err != nil {
		return nil, err
	}
	job, err := h.app.Services.Jobs.Stop(ctx, actor, id)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.StopJobResponse{Job: present.ProtoJob(job)}, nil
}

func (h *taskConnectHandler) ListJobEvents(
	ctx context.Context,
	req *factoryv1.ListJobEventsRequest,
) (*factoryv1.ListJobEventsResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	events, err := h.app.Services.Jobs.ListEvents(ctx, actor, id, int(req.GetAfterSeq()))
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.ListJobEventsResponse{Events: present.ProtoJobEvents(events)}, nil
}

// claimedJobToProto is the job as the runner receives it.
func claimedJobToProto(c *services.ClaimedJob) *factoryv1.ClaimedJob {
	timeLimit := int32(c.Repo.TimeLimitMinutes) //nolint:gosec // minutes, far below 2^31
	taskID := ""
	chatID := ""
	if c.ChatID != uuid.Nil {
		chatID = c.ChatID.String()
	}
	if !c.Job.IsRoutine() && !c.Job.IsChat() {
		taskID = models.TaskRef(c.Job.TaskNumber)
	}
	return &factoryv1.ClaimedJob{
		Id:               c.Job.ID.String(),
		TaskId:           taskID,
		Routine:          c.Job.Routine,
		RepoOwner:        c.Repo.Owner,
		RepoName:         c.Repo.Name,
		DefaultBranch:    c.Repo.DefaultBranch,
		Prompt:           c.Job.Prompt,
		Branch:           c.Job.Branch,
		TimeLimitMinutes: timeLimit,
		McpClientId:      c.MCPClientID,
		McpClientSecret:  c.MCPClientSecret,
		ResumeSessionId:  c.ResumeSessionID,
		ChatId:           chatID,
	}
}

func (h *runnerHandler) ClaimJob(
	ctx context.Context,
	req *factoryv1.ClaimJobRequest,
) (*factoryv1.ClaimJobResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	claimed, err := h.app.Services.Jobs.ClaimJob(
		ctx, runner.ID, int(req.GetMaxJobs()), harnessFromProto(req.GetHarness()),
		req.GetModel(), h.app.HeartbeatHold,
	)
	if err != nil {
		return nil, present.MapError(err)
	}
	if claimed == nil {
		return &factoryv1.ClaimJobResponse{}, nil
	}
	return &factoryv1.ClaimJobResponse{Job: claimedJobToProto(claimed)}, nil
}

func (h *runnerHandler) CheckIn(
	ctx context.Context,
	req *factoryv1.CheckInRequest,
) (*factoryv1.CheckInResponse, error) {
	if _, err := h.runnerFrom(ctx); err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	stop, err := h.app.Services.Jobs.CheckIn(ctx, id, req.GetSessionId())
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.CheckInResponse{Stop: stop}, nil
}

func (h *runnerHandler) AppendJobEvents(
	ctx context.Context,
	req *factoryv1.AppendJobEventsRequest,
) (*factoryv1.AppendJobEventsResponse, error) {
	if _, err := h.runnerFrom(ctx); err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	lines := make([]models.JobLine, len(req.GetLines()))
	for i, l := range req.GetLines() {
		lines[i] = models.JobLine{Seq: int(l.GetSeq()), Line: l.GetLine()}
	}
	if err = h.app.Services.Jobs.AppendEvents(ctx, id, lines); err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.AppendJobEventsResponse{}, nil
}

func (h *runnerHandler) GetPushToken(
	ctx context.Context,
	req *factoryv1.GetPushTokenRequest,
) (*factoryv1.GetPushTokenResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	token, err := h.app.Services.Jobs.PushToken(ctx, runner.ID, id)
	switch {
	case errors.Is(err, services.ErrPushNotConfigured),
		errors.Is(err, services.ErrNotInstalled):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error())
	case errors.Is(err, services.ErrGitHubFailed):
		// Retryable, and none of GitHub's own text goes to the runner.
		h.app.Logger.WarnContext(ctx, "could not mint a push token", "error", err)
		return nil, connect.NewError(connect.CodeUnavailable,
			services.ErrGitHubFailed.Error())
	case err != nil:
		return nil, present.MapError(err)
	}
	return &factoryv1.GetPushTokenResponse{
		Token:     token.Value,
		ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// maxFinishedQuery bounds one ListFinishedTasks call.
const maxFinishedQuery = 500

func (h *runnerHandler) ListFinishedTasks(
	ctx context.Context,
	req *factoryv1.ListFinishedTasksRequest,
) (*factoryv1.ListFinishedTasksResponse, error) {
	if _, err := h.runnerFrom(ctx); err != nil {
		return nil, err
	}
	if len(req.GetTaskIds()) > maxFinishedQuery {
		return nil, connect.NewError(connect.CodeInvalidArgument, "too many task ids")
	}
	ids := make([]uuid.UUID, len(req.GetTaskIds()))
	asked := make(map[uuid.UUID]string, len(ids))
	for i, raw := range req.GetTaskIds() {
		id, err := h.app.taskID(ctx, raw)
		if err != nil {
			return nil, err
		}
		ids[i] = id
		asked[id] = raw
	}
	done, err := h.app.Services.Jobs.FinishedTasks(ctx, ids)
	if err != nil {
		return nil, present.MapError(err)
	}
	// Answer in the references the runner asked with.
	out := make([]string, len(done))
	for i, id := range done {
		out[i] = asked[id]
	}
	return &factoryv1.ListFinishedTasksResponse{TaskIds: out}, nil
}

func (h *runnerHandler) CompleteJob(
	ctx context.Context,
	req *factoryv1.CompleteJobRequest,
) (*factoryv1.CompleteJobResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	var resetsAt *time.Time
	if at, parseErr := time.Parse(time.RFC3339, req.GetLimitResetsAt()); parseErr == nil {
		resetsAt = &at
	}
	_, err = h.app.Services.Jobs.Complete(
		ctx, runner.ID, id, req.GetSucceeded(), req.GetOutcome(), req.GetSessionId(),
		req.GetPrUrl(), req.GetLimited(), resetsAt,
	)
	if err != nil {
		return nil, present.MapError(err)
	}
	// The job is already over, so a report that fails must not fail the call.
	_ = h.app.Services.Jobs.ReportBlocked(
		ctx, runner.ID, id, req.GetBlocked(), req.GetWaitingOn())
	if fh := req.GetFinishedHarness(); fh != factoryv1.Harness_HARNESS_UNSPECIFIED {
		_ = h.app.Services.Jobs.RecordFinishedBy(
			ctx, runner.ID, id, harnessFromProto(fh), req.GetFinishedModel())
	}
	if t := req.GetTimings(); t != nil {
		_ = h.app.Services.Jobs.RecordTimings(ctx, runner.ID, id, models.JobTimings{
			Agent: t.GetAgentSeconds(), Gates: t.GetGatesSeconds(),
			Publish: t.GetPublishSeconds(),
			CI:      t.GetCiSeconds(), Merge: t.GetMergeSeconds(), Total: t.GetTotalSeconds(),
			GateRetries: int(t.GetGateRetries()), PushRetries: int(t.GetPushRetries()),
			CIRetries: int(t.GetCiRetries()),
		}, req.GetSucceeded() && !req.GetLimited())
	}
	return &factoryv1.CompleteJobResponse{}, nil
}

func (h *runnerHandler) AddJobComment(
	ctx context.Context,
	req *factoryv1.AddJobCommentRequest,
) (*factoryv1.AddJobCommentResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	err = h.app.Services.Jobs.AddProgressNote(ctx, runner.ID, id, req.GetBody())
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.AddJobCommentResponse{}, nil
}

func (h *runnerHandler) EndChat(
	ctx context.Context,
	req *factoryv1.EndChatRequest,
) (*factoryv1.EndChatResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetChatId())
	if err != nil {
		return nil, err
	}
	if _, err = h.app.Services.Chats.End(ctx, runner.ID, id, req.GetReport()); err != nil {
		return nil, present.MapError(err)
	}
	if h.app.Chats != nil {
		h.app.Chats.Publish(id)
	}
	return &factoryv1.EndChatResponse{}, nil
}
