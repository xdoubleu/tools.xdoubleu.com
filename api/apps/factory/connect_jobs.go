package factory

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
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
	job, err := h.app.services.Jobs.Run(ctx, actor, id, req.GetText())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RunTaskResponse{Job: protoJob(job)}, nil
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
	}
}

func (h *runnerHandler) ClaimJob(
	ctx context.Context,
	_ *factoryv1.ClaimJobRequest,
) (*factoryv1.ClaimJobResponse, error) {
	if _, err := h.runnerFrom(ctx); err != nil {
		return nil, err
	}
	// Held open so the runner needs no inbound port and learns of a job within a
	// second of it being queued; answered empty when the hold ends.
	deadline := time.Now().Add(h.app.heartbeatHold)
	for {
		claimed, err := h.app.services.Jobs.Claim(ctx)
		if err != nil {
			return nil, mapError(err)
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
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.CompleteJobResponse{}, nil
}
