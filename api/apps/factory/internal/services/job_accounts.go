package services

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/agentaccounts"
)

const shortJobIDLen = 8

// JobAccounts makes and ends the agent account a job runs as (adr-0039). It is
// optional: without it a job gets no MCP identity.
type JobAccounts interface {
	CreateForJob(
		ctx context.Context,
		createdBy, name string,
		scope agentaccounts.Scope,
	) (*agentaccounts.Account, string, error)
	RevokeForJob(ctx context.Context, jobID uuid.UUID) error
	ActiveJobs(ctx context.Context) ([]uuid.UUID, error)
	ScopeOf(ctx context.Context, userID uuid.UUID) (*agentaccounts.Scope, error)
}

// issueAccount gives a claimed job its own agent account and puts the client
// credentials on the claim. A job that can't have one is failed, not run without.
func (s *JobService) issueAccount(
	ctx context.Context,
	claimed *ClaimedJob,
	runnerID uuid.UUID,
) error {
	job := &claimed.Job
	name := "job #" + job.ID.String()[:shortJobIDLen] + " · " +
		string(job.Harness)
	account, secret, err := s.accounts.CreateForJob(ctx, systemAuthor, name,
		agentaccounts.Scope{JobID: job.ID, TaskID: job.TaskID})
	if err != nil {
		// The reason names no internals: it is posted on the task.
		reason := "Could not create the job's agent account."
		if _, failErr := s.jobs.Complete(
			ctx, job.ID, models.JobFailed, reason, "", "", runnerID,
		); failErr == nil {
			_ = s.finishTask(
				ctx, job.TaskID, systemAuthor, "Job failed: "+reason, models.StatusNeedsYou,
			)
		}
		return err
	}
	claimed.MCPClientID, claimed.MCPClientSecret = account.ClientID, secret
	return nil
}

// revokeAccount ends a finished job's account. A failure is left for the sweep,
// which runs on the next claim and the next board read.
func (s *JobService) revokeAccount(ctx context.Context, jobID uuid.UUID) {
	if s.accounts != nil {
		_ = s.accounts.RevokeForJob(ctx, jobID)
	}
}

// sweepAccounts ends the accounts of jobs that are no longer running: one a
// revoke missed, or a job that was lost or stopped. The secret stays valid until
// then, so this runs wherever jobs are swept.
func (s *JobService) sweepAccounts(ctx context.Context) {
	if s.accounts == nil {
		return
	}
	active, err := s.accounts.ActiveJobs(ctx)
	if err != nil || len(active) == 0 {
		return
	}
	running, err := s.jobs.Running(ctx, active)
	if err != nil {
		return
	}
	for _, id := range active {
		if !slices.Contains(running, id) {
			_ = s.accounts.RevokeForJob(ctx, id)
		}
	}
}

// TaskScope is the one task the user's account may act on, or nil when it has
// no such limit (a person's account, or none).
func (s *JobService) TaskScope(
	ctx context.Context,
	userID uuid.UUID,
) (*uuid.UUID, error) {
	if s.accounts == nil {
		return nil, nil //nolint:nilnil // no scope is an answer, not an error
	}
	scope, err := s.accounts.ScopeOf(ctx, userID)
	if err != nil || scope == nil {
		return nil, err
	}
	return &scope.TaskID, nil
}
