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

// JobKind tells a task's or routine's job from a chat session's.
type JobKind string

const (
	JobKindTask JobKind = "task"
	// JobKindChat is a chat session's job: created running, pinned to a runner.
	JobKindChat JobKind = "chat"
)

// Job is one run of a harness on a task, on the runner's Mac.
type Job struct {
	ID uuid.UUID
	// Kind is empty on a job to create, which makes it a task job.
	Kind JobKind
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
	// FinishedHarness and FinishedModel are what finished the job after it
	// continued on the fallback; empty when the first harness did.
	FinishedHarness Harness
	FinishedModel   string
	CreatedBy       string
	CreatedAt       time.Time
	// StartedAt, LastCheckIn and FinishedAt are nil until they happen.
	StartedAt   *time.Time
	LastCheckIn *time.Time
	FinishedAt  *time.Time
}

// IsChat reports whether the job holds a chat session's runner slot.
func (j *Job) IsChat() bool {
	return j.Kind == JobKindChat
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
	// UpdatedBy is who last saved it, as "<kind>:<id>".
	UpdatedBy string
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

// JobTimings is where a job's time went, in seconds, and how many attempts were
// repeated by the stage that failed.
type JobTimings struct {
	Agent, Gates, Publish, CI, Merge, Total float64
	GateRetries, PushRetries, CIRetries     int
}

// FailureCounts are the counts behind the factory failure gauges.
type FailureCounts struct {
	RedPullRequests   int
	JobsWithoutPR     int
	NoPRTasks         []string
	UnverifiedOutcome int
	StuckPullRequests int
}

// ChatState is where a chat session stands.
type ChatState string

const (
	// ChatIdle waits for the person's next message.
	ChatIdle ChatState = "idle"
	// ChatWorking is the harness at work on a message.
	ChatWorking ChatState = "working"
	// ChatAsking waits for the person to answer a question the harness asked.
	ChatAsking  ChatState = "asking"
	ChatStopped ChatState = "stopped"
)

// ChatSession is an interactive session between a person and a harness on one
// runner.
type ChatSession struct {
	ID     uuid.UUID
	RepoID uuid.UUID
	// TaskID is uuid.Nil for a session about no task.
	TaskID  uuid.UUID
	Harness Harness
	// RunnerID is the runner the session is pinned to.
	RunnerID uuid.UUID
	// JobID is the chat job that holds the runner's slot.
	JobID uuid.UUID
	State ChatState
	// Branch is the worktree branch the session commits to.
	Branch string
	// Report is what the session left behind when it stopped.
	Report    string
	LastSeq   int
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	StoppedAt *time.Time
}

// ChatEvent is one numbered entry of a chat session's transcript.
type ChatEvent struct {
	SessionID uuid.UUID
	Seq       int
	Kind      string
	Payload   string
	CreatedAt time.Time
}

// ChatEventInput is an event to append; the store numbers it.
type ChatEventInput struct {
	Kind    string
	Payload string
}

// The kinds of event the service itself writes; the runner adds its own.
const (
	ChatEventMessage = "user_message"
	ChatEventAnswer  = "user_answer"
	ChatEventStopped = "stopped"
)
