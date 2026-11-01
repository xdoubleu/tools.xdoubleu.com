package models

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus is where a job is in its life.
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
	// JobLost is a running job that stopped checking in.
	JobLost JobStatus = "lost"
)

// Finished reports whether the job can no longer change.
func (s JobStatus) Finished() bool {
	return s == JobSucceeded || s == JobFailed || s == JobCancelled || s == JobLost
}

// Job is one run of a harness on a task, on the runner's Mac.
type Job struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	RepoID    uuid.UUID
	Harness   Harness
	Prompt    string
	Status    JobStatus
	Branch    string
	SessionID string
	Outcome   string
	CreatedBy string
	CreatedAt time.Time
	// StartedAt, LastCheckIn and FinishedAt are nil until they happen.
	StartedAt   *time.Time
	LastCheckIn *time.Time
	FinishedAt  *time.Time
}
