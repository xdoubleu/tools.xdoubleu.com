package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// parkedResumer is the slice of services.JobService the job needs.
type parkedResumer interface {
	ResumeParked(ctx context.Context) (int, error)
	ResumeWaiting(ctx context.Context) (int, error)
}

// ResumeParkedJob resumes tasks parked on the sandbox once a new job image ships,
// and tasks whose waited-on tasks have merged.
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
	w, waitErr := j.jobs.ResumeWaiting(ctx)
	if w > 0 {
		logger.InfoContext(ctx, "resumed tasks whose wait is over", "tasks", w)
	}
	return errors.Join(err, waitErr) //nolint:wrapcheck // the job runner names the job
}
