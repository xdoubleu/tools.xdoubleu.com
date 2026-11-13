package threading

import (
	"context"
	"log/slog"
	"time"
)

// CallbackFunc is called before and after running a job.
type CallbackFunc = func(id string, isRunning bool, lastRunTime *time.Time)

// Job describes the interface for a job executable by a [WorkerPool].
type Job interface {
	ID() string
	Run(context.Context, *slog.Logger) error
}

// Scheduled is a Job run on jobqueue.JobQueue's tick; other Jobs only run via
// ForceRun.
type Scheduled interface {
	Job
	RunEvery() time.Duration
}
