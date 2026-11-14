package jobs

import (
	"context"
	"log/slog"
	"time"
)

// parkedResumer is the slice of services.JobService the job needs.
type parkedResumer interface {
	ResumeParked(ctx context.Context) (int, error)
}

// ResumeParkedJob resumes tasks parked on the sandbox once a new job image ships.
type ResumeParkedJob struct {
	jobs parkedResumer
}

func NewResumeParkedJob(jobs parkedResumer) ResumeParkedJob {
	return ResumeParkedJob{jobs: jobs}
}

func (j ResumeParkedJob) ID() string {
	return "factory-resume-parked"
}

func (j ResumeParkedJob) RunEvery() time.Duration {
	const minutes = 5
	return minutes * time.Minute
}

func (j ResumeParkedJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.jobs.ResumeParked(ctx)
	if n > 0 {
		logger.InfoContext(ctx, "resumed parked tasks", "tasks", n)
	}
	return err //nolint:wrapcheck // the job runner names the job
}
