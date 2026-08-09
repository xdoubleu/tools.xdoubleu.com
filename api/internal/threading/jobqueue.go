package threading

import (
	"context"
	"log/slog"
	"time"
)

// CallbackFunc describes the interface for a func
// called before and after running a job.
type CallbackFunc = func(id string, isRunning bool, lastRunTime *time.Time)

// Job describes the interface for a job executable by a [WorkerPool].
type Job interface {
	ID() string
	Run(context.Context, *slog.Logger) error
}

// Scheduled is a Job that runs on a recurring schedule via
// jobqueue.JobQueue's periodic tick. A Job that doesn't implement Scheduled
// is trigger-only: only an explicit JobQueue.ForceRun runs it.
type Scheduled interface {
	Job
	RunEvery() time.Duration
}
