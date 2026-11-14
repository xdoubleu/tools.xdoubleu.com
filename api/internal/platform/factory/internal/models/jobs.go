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
	ID uuid.UUID
	// TaskID is uuid.Nil for a routine's job, which has no task.
	TaskID uuid.UUID
	// Routine names the routine a task-less job runs; empty for a task's job.
	Routine string
	// TaskNumber is the task's human-readable number.
	TaskNumber int64
	RepoID     uuid.UUID
	Harness    Harness
	// Model is the OpenRouter model of an OpenCode job; empty for Claude Code.
	Model     string
	Prompt    string
	Status    JobStatus
	Branch    string
	SessionID string
	Outcome   string
	// ParentJobID is the job a Reply resumes; nil for a job started by Run.
	ParentJobID *uuid.UUID
	// PRURL is the pull request the job opened, if it did.
	PRURL string
	// RunnerID is the runner that claimed the job; nil until it is claimed.
	RunnerID *uuid.UUID
	// PRState, PRCI and PRReview are where the pull request stands, as last read
	// from GitHub; empty until it has been read.
	PRState  string
	PRCI     string
	PRReview string
	// Limited says the harness's provider stopped the job on a usage limit;
	// LimitResetsAt is when it lifts, nil if unsaid.
	Limited       bool
	LimitResetsAt *time.Time
	CreatedBy     string
	CreatedAt     time.Time
	// StartedAt, LastCheckIn and FinishedAt are nil until they happen.
	StartedAt   *time.Time
	LastCheckIn *time.Time
	FinishedAt  *time.Time
}

// IsRoutine reports whether the job belongs to a routine, not a task.
func (j *Job) IsRoutine() bool {
	return j.Routine != ""
}

// Routine is a named job the server starts on a schedule, with no task.
type Routine struct {
	Name    string
	RepoID  uuid.UUID
	Enabled bool
	// IntervalMinutes is the time between scheduled runs; 0 runs it by hand only.
	IntervalMinutes int
	CreatedAt       time.Time
	// LastJob is the routine's newest job; nil until it has run.
	LastJob *Job
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
