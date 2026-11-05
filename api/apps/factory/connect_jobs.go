package factory

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/githubapp"
)

// claimPoll is how often a held ClaimJob looks for a queued job.
const claimPoll = time.Second

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

func protoJob(j *models.Job) *factoryv1.Job {
	return &factoryv1.Job{
		Id:        j.ID.String(),
		TaskId:    j.TaskID.String(),
		Status:    jobStatusToProto(j.Status),
		Branch:    j.Branch,
		CreatedAt: j.CreatedAt.Format(time.RFC3339),
		PrUrl:     j.PRURL,
		PrState:   j.PRState,
		PrCi:      j.PRCI,
		PrReview:  j.PRReview,
		Harness:   harnessToProto(j.Harness),
		Model:     j.Model,
	}
}

func (h *taskConnectHandler) RunTask(
	ctx context.Context,
	req *factoryv1.RunTaskRequest,
) (*factoryv1.RunTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	job, err := h.app.services.Jobs.Run(
		ctx,
		actor,
		id,
		req.GetText(),
		services.HarnessPick{
			Harness: harnessFromProtoOptional(req.GetHarness()),
			Model:   req.GetModel(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RunTaskResponse{Job: protoJob(job)}, nil
}

func (h *taskConnectHandler) ReplyTask(
	ctx context.Context,
	req *factoryv1.ReplyTaskRequest,
) (*factoryv1.ReplyTaskResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	job, err := h.app.services.Jobs.Reply(ctx, actor, id, req.GetText())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.ReplyTaskResponse{Job: protoJob(job)}, nil
}

func (h *taskConnectHandler) StopJob(
	ctx context.Context,
	req *factoryv1.StopJobRequest,
) (*factoryv1.StopJobResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	job, err := h.app.services.Jobs.Stop(ctx, actor, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.StopJobResponse{Job: protoJob(job)}, nil
}

func (h *taskConnectHandler) ListJobEvents(
	ctx context.Context,
	req *factoryv1.ListJobEventsRequest,
) (*factoryv1.ListJobEventsResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	events, err := h.app.services.Jobs.ListEvents(ctx, actor, id, int(req.GetAfterSeq()))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*factoryv1.JobEvent, len(events))
	for i := range events {
		out[i] = &factoryv1.JobEvent{
			Seq:       int32(events[i].Seq), //nolint:gosec // capped per job
			Kind:      events[i].Kind,
			Payload:   events[i].Payload,
			CreatedAt: events[i].CreatedAt.Format(time.RFC3339),
		}
	}
	return &factoryv1.ListJobEventsResponse{Events: out}, nil
}

// claimedJobToProto is the job as the runner receives it.
func claimedJobToProto(c *services.ClaimedJob) *factoryv1.ClaimedJob {
	timeLimit := int32(c.Repo.TimeLimitMinutes) //nolint:gosec // minutes, far below 2^31
	return &factoryv1.ClaimedJob{
		Id:               c.Job.ID.String(),
		TaskId:           c.Job.TaskID.String(),
		RepoOwner:        c.Repo.Owner,
		RepoName:         c.Repo.Name,
		DefaultBranch:    c.Repo.DefaultBranch,
		Harness:          harnessToProto(c.Job.Harness),
		Prompt:           c.Job.Prompt,
		Branch:           c.Job.Branch,
		SetupCommands:    c.Repo.SetupCommands,
		TimeLimitMinutes: timeLimit,
		McpClientId:      c.MCPClientID,
		McpClientSecret:  c.MCPClientSecret,
		ResumeSessionId:  c.ResumeSessionID,
		Model:            c.Job.Model,
	}
}

func (h *runnerHandler) ClaimJob(
	ctx context.Context,
	_ *factoryv1.ClaimJobRequest,
) (*factoryv1.ClaimJobResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	// A claiming runner is a live one, whether or not its heartbeat is running.
	if err = h.app.services.Runners.Heartbeat(ctx, runner.ID, ""); err != nil {
		return nil, mapError(err)
	}
	// Held open so the runner needs no inbound port and learns of a job within a
	// second of it being queued; answered empty when the hold ends.
	deadline := time.Now().Add(h.app.heartbeatHold)
	for {
		claimed, claimErr := h.app.services.Jobs.Claim(ctx, runner.ID)
		if claimErr != nil {
			return nil, mapError(claimErr)
		}
		if claimed != nil {
			return &factoryv1.ClaimJobResponse{Job: claimedJobToProto(claimed)}, nil
		}
		// A negative wait fires at once, so the last look happens after the hold.
		select {
		case <-time.After(min(claimPoll, time.Until(deadline))):
		case <-ctx.Done():
			return &factoryv1.ClaimJobResponse{}, nil
		}
		if time.Now().After(deadline) {
			return &factoryv1.ClaimJobResponse{}, nil
		}
	}
}

func (h *runnerHandler) CheckIn(
	ctx context.Context,
	req *factoryv1.CheckInRequest,
) (*factoryv1.CheckInResponse, error) {
	if _, err := h.runnerFrom(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	stop, err := h.app.services.Jobs.CheckIn(ctx, id, req.GetSessionId())
	if err != nil {
		return nil, mapError(err)
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
	id, err := parseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	lines := make([]models.JobLine, len(req.GetLines()))
	for i, l := range req.GetLines() {
		lines[i] = models.JobLine{Seq: int(l.GetSeq()), Line: l.GetLine()}
	}
	if err = h.app.services.Jobs.AppendEvents(ctx, id, lines); err != nil {
		return nil, mapError(err)
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
	id, err := parseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	token, err := h.app.services.Jobs.PushToken(ctx, runner.ID, id)
	switch {
	case errors.Is(err, services.ErrPushNotConfigured),
		errors.Is(err, githubapp.ErrNotInstalled):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error())
	case errors.Is(err, services.ErrGitHubFailed):
		// Retryable, and none of GitHub's own text goes to the runner.
		return nil, connect.NewError(connect.CodeUnavailable, err.Error())
	case err != nil:
		return nil, mapError(err)
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
	for i, raw := range req.GetTaskIds() {
		id, err := parseID(raw)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	done, err := h.app.services.Jobs.FinishedTasks(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]string, len(done))
	for i, id := range done {
		out[i] = id.String()
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
	id, err := parseID(req.GetJobId())
	if err != nil {
		return nil, err
	}
	_, err = h.app.services.Jobs.Complete(
		ctx, runner.ID, id, req.GetSucceeded(), req.GetOutcome(), req.GetSessionId(),
		req.GetPrUrl(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.CompleteJobResponse{}, nil
}
