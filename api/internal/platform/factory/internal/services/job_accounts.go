package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
)

const (
	shortJobIDLen = 8
	humanPrefix   = string(models.ActorHuman) + ":"
	agentPrefix   = string(models.ActorAgent) + ":"
)

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
	// OwnerNames maps each account's user to the name of the person who owns it,
	// or "" when that person can't be resolved. Unknown users are left out.
	OwnerNames(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error)
}

// jobOwner is the person responsible for a job's agent account: whoever started
// the job. Only a job no person started (auto-pickup, a requeue, a routine)
// falls back to the factory itself.
func jobOwner(job *models.Job) string {
	if strings.HasPrefix(job.CreatedBy, humanPrefix) {
		return job.CreatedBy
	}
	return systemAuthor
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
	account, secret, err := s.accounts.CreateForJob(ctx, jobOwner(job), name,
		agentaccounts.Scope{JobID: job.ID, TaskID: job.TaskID})
	if err != nil {
		// The reason names no internals: it is posted on the task.
		reason := "Could not create the job's agent account."
		if _, failErr := s.jobs.Complete(
			ctx, job.ID, models.JobFailed, reason, "", "", runnerID, false, nil,
		); failErr == nil && !job.IsRoutine() {
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

// JobOf is the job the user's account was made for, or nil when it is not a
// job's account.
func (s *JobService) JobOf(
	ctx context.Context,
	userID uuid.UUID,
) (*models.Job, error) {
	if s.accounts == nil {
		return nil, nil //nolint:nilnil // no job is an answer, not an error
	}
	scope, err := s.accounts.ScopeOf(ctx, userID)
	if err != nil || scope == nil {
		return nil, err
	}
	return s.jobs.Get(ctx, scope.JobID)
}

// ActorLabels maps each agent actor ("agent:<user id>") to the actor followed by
// its owner, so a reader sees which person the agent acts for. An owner that
// can't be resolved reads "unknown owner" rather than nothing. Other actors are
// left out.
func (s *JobService) ActorLabels(
	ctx context.Context,
	actors []string,
) map[string]string {
	out := map[string]string{}
	if s.accounts == nil {
		return out
	}
	ids := []uuid.UUID{}
	byID := map[uuid.UUID]string{}
	for _, actor := range actors {
		id, err := uuid.Parse(strings.TrimPrefix(actor, agentPrefix))
		if !strings.HasPrefix(actor, agentPrefix) || err != nil {
			continue
		}
		if _, seen := byID[id]; !seen {
			byID[id] = actor
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return out
	}
	// A failed lookup labels every agent with an unknown owner.
	names, _ := s.accounts.OwnerNames(ctx, ids)
	for id, actor := range byID {
		owner := names[id]
		if owner == "" {
			owner = "unknown owner"
		}
		out[actor] = actor + " (agent of " + owner + ")"
	}
	return out
}
