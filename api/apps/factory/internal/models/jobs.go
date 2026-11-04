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
	// PRURL is the pull request the job opened, if it did.
	PRURL string
	// RunnerID is the runner that claimed the job; nil until it is claimed.
	RunnerID *uuid.UUID
	// PRState, PRCI and PRReview are where the pull request stands, as last read
	// from GitHub; empty until it has been read.
	PRState   string
	PRCI      string
	PRReview  string
	CreatedBy string
	CreatedAt time.Time
	// StartedAt, LastCheckIn and FinishedAt are nil until they happen.
	StartedAt   *time.Time
	LastCheckIn *time.Time
	FinishedAt  *time.Time
}

// JobEvent is one line of a job's log.
type JobEvent struct {
	JobID     uuid.UUID
	Seq       int
	Kind      string
	Payload   string
	CreatedAt time.Time
}

// JobLine is one numbered line of harness output, as the runner sends it.
type JobLine struct {
	Seq  int
	Line string
}
