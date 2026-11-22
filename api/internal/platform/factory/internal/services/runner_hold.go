package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	// claimPoll is how often a held ClaimJob looks for a queued job.
	claimPoll = time.Second
	// maxRunnerJobs bounds what a runner may ask for.
	maxRunnerJobs = 8
)

// HoldHeartbeat records that the runner is alive, holds the call open for hold so
// the runner needs no inbound port, then records it again and returns how many
// jobs wait for a runner. A hold cut short by ctx returns 0.
func (s *JobService) HoldHeartbeat(
	ctx context.Context,
	runnerID uuid.UUID,
	version string,
	repos []string,
	reported bool,
	hold time.Duration,
) (int, error) {
	if err := s.runners.Touch(ctx, runnerID, version, repos, reported); err != nil {
		return 0, err
	}
	timer := time.NewTimer(hold)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return 0, nil
	}
	if err := s.runners.Touch(ctx, runnerID, version, repos, reported); err != nil {
		return 0, err
	}
	// A failed count must not fail the heartbeat that keeps the runner online; the
	// watchdog reads 0 as nothing waiting.
	queued, _ := s.QueuedCount(ctx)
	return queued, nil
}

// ClaimJob starts the oldest queued job for the runner, polling for up to hold so
// the runner learns of a job within a second of it being queued. It returns nil
// when the hold ends, or ctx is done, with nothing to claim.
func (s *JobService) ClaimJob(
	ctx context.Context,
	runnerID uuid.UUID,
	maxJobs int,
	harness models.Harness,
	model string,
	hold time.Duration,
) (*ClaimedJob, error) {
	// A claiming runner is a live one, whether or not its heartbeat is running.
	if err := s.runners.Touch(ctx, runnerID, "", nil, false); err != nil {
		return nil, err
	}
	deadline := s.now().Add(hold)
	maxJobs = min(maxJobs, maxRunnerJobs)
	// The sweep and pick-up bookkeeping runs once per hold, not on every poll.
	if err := s.PrepareClaim(ctx, runnerID, maxJobs, harness, model); err != nil {
		return nil, err
	}
	for {
		claimed, err := s.ClaimQueued(ctx, runnerID, maxJobs, harness, model)
		if err != nil || claimed != nil {
			return claimed, err
		}
		// A negative wait fires at once, so the last look happens after the hold.
		select {
		case <-time.After(min(claimPoll, deadline.Sub(s.now()))):
		case <-ctx.Done():
			return nil, nil //nolint:nilnil // an empty hold is an answer, not an error
		}
		if s.now().After(deadline) {
			return nil, nil //nolint:nilnil // an empty hold is an answer, not an error
		}
	}
}
