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
// RunEvery <= 0 marks the job trigger-only: jobqueue.JobQueue's periodic
// tick never runs it, only an explicit JobQueue.ForceRun does.
type Job interface {
	ID() string
	Run(context.Context, *slog.Logger) error
	RunEvery() time.Duration
}
