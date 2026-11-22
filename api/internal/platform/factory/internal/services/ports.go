package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// The storage each service uses, declared here and implemented by
// internal/repositories.

// TaskStore is the storage of tasks and their comments.
type TaskStore interface {
	AddComment(
		ctx context.Context,
		taskID uuid.UUID,
		author string,
		body string,
	) (*models.Comment, error)
	BlockPickup(
		ctx context.Context,
		id uuid.UUID,
		reason string,
	) (*models.Task, error)
	ByKeys(
		ctx context.Context,
		ids []uuid.UUID,
		numbers []int64,
	) ([]models.Task, error)
	Comments(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]models.Comment, error)
	Create(
		ctx context.Context,
		t models.Task,
		actor models.Actor,
	) (*models.Task, error)
	Find(
		ctx context.Context,
		f models.TaskFilter,
	) ([]models.Task, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	List(
		ctx context.Context,
		query string,
	) ([]models.Task, error)
	MergedBetween(
		ctx context.Context,
		from, to time.Time,
	) ([]models.Task, error)
	Move(
		ctx context.Context,
		id uuid.UUID,
		status models.Status,
	) (*models.Task, error)
	NextReady(ctx context.Context) (*models.PickupCandidate, error)
	NoteAutoPickupOff(ctx context.Context) error
	Resolve(ctx context.Context, ref string) (uuid.UUID, error)
	SetLabels(
		ctx context.Context,
		id uuid.UUID,
		labels []string,
	) error
	SetMergeSHA(
		ctx context.Context,
		id uuid.UUID,
		sha string,
	) error
	SetWaitingOn(
		ctx context.Context,
		id uuid.UUID,
		numbers []int64,
	) error
	Statuses(
		ctx context.Context,
		ids []uuid.UUID,
	) (map[uuid.UUID]models.Status, error)
	Update(
		ctx context.Context,
		t models.Task,
		actor models.Actor,
	) (*models.Task, error)
	WaitOver(ctx context.Context) ([]uuid.UUID, error)
}

// RepoStore is the storage of registered repos.
type RepoStore interface {
	Create(
		ctx context.Context,
		repo models.Repo,
	) (*models.Repo, error)
	Get(
		ctx context.Context,
		id uuid.UUID,
	) (*models.Repo, error)
	List(ctx context.Context) ([]models.Repo, error)
	Update(
		ctx context.Context,
		repo models.Repo,
	) (*models.Repo, error)
}

// RunnerStore is the storage of runners.
type RunnerStore interface {
	ByToken(
		ctx context.Context,
		tokenHash string,
	) (*models.Runner, error)
	Create(
		ctx context.Context,
		name, createdBy, pairingHash string,
		expiresAt time.Time,
	) (*models.Runner, error)
	List(ctx context.Context) ([]models.Runner, error)
	Pair(
		ctx context.Context,
		pairingHash, tokenHash, version string,
	) (*models.Runner, error)
	Revoke(
		ctx context.Context,
		id uuid.UUID,
	) (*models.Runner, error)
	SetMaxJobs(
		ctx context.Context,
		id uuid.UUID,
		maxJobs int,
	) error
	Touch(
		ctx context.Context,
		id uuid.UUID,
		version string,
		repos []string,
		reported bool,
	) error
}

// JobStore is the storage of jobs.
type JobStore interface {
	ActiveLimit(ctx context.Context) (*time.Time, error)
	AnyRunning(ctx context.Context) (bool, error)
	ByTask(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]models.Job, error)
	CancelForTask(
		ctx context.Context,
		taskID uuid.UUID,
	) (*models.Job, error)
	CancelRoutine(
		ctx context.Context,
		routine string,
	) (*models.Job, error)
	CheckIn(
		ctx context.Context,
		id uuid.UUID,
		sessionID string,
	) error
	Claim(
		ctx context.Context,
		runnerID uuid.UUID,
		maxJobs int,
		harness models.Harness,
		model string,
		skipClaude bool,
	) (*models.Job, error)
	Complete(
		ctx context.Context,
		id uuid.UUID,
		status models.JobStatus,
		outcome, sessionID, prURL string,
		runnerID uuid.UUID,
		limited bool,
		limitResetsAt *time.Time,
	) (*models.Job, error)
	CountQueued(ctx context.Context) (int, error)
	Create(
		ctx context.Context,
		j models.Job,
	) (*models.Job, error)
	ExpireLost(
		ctx context.Context,
		cutoff time.Time,
	) ([]models.Job, error)
	FailureCounts(
		ctx context.Context,
		since, stuckBefore time.Time,
	) (models.FailureCounts, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Job, error)
	LastForRoutine(
		ctx context.Context,
		routine string,
	) (*models.Job, error)
	MarkPullRequestPolled(
		ctx context.Context,
		id uuid.UUID,
	) error
	OldestQueued(ctx context.Context) (*models.Job, error)
	RecordTimings(
		ctx context.Context,
		id, runnerID uuid.UUID,
		t models.JobTimings,
	) error
	Running(
		ctx context.Context,
		ids []uuid.UUID,
	) ([]uuid.UUID, error)
	SetPullRequestStatus(
		ctx context.Context,
		id uuid.UUID,
		state, ci, review string,
	) error
	WithOpenPullRequest(
		ctx context.Context,
		limit int,
	) ([]models.Job, error)
	WithPullRequest(
		ctx context.Context,
		ids []uuid.UUID,
	) (map[uuid.UUID]bool, error)
}

// JobEventStore is the storage of job events.
type JobEventStore interface {
	After(
		ctx context.Context,
		jobID uuid.UUID,
		afterSeq, limit int,
	) ([]models.JobEvent, error)
	Append(
		ctx context.Context,
		jobID uuid.UUID,
		seqs []int32,
		kinds, payloads []string,
	) error
	DeleteBefore(
		ctx context.Context,
		cutoff time.Time,
	) (int64, error)
	Last(
		ctx context.Context,
		jobID uuid.UUID,
		limit int,
	) ([]models.JobEvent, error)
}

// SettingsStore is the storage of factory settings.
type SettingsStore interface {
	ImageDigest(ctx context.Context) (string, error)
	PathRulesBackfilled(ctx context.Context) (bool, error)
	PickupPaused(ctx context.Context) (bool, error)
	SetImageDigest(ctx context.Context, digest string) error
	SetPathRulesBackfilled(ctx context.Context) error
	SetPickupPaused(ctx context.Context, paused bool) error
}

// LinkedPullRequestStore is the storage of pull requests linked to tasks.
type LinkedPullRequestStore interface {
	ByTask(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]models.LinkedPullRequest, error)
	Open(
		ctx context.Context,
		limit int,
	) ([]models.LinkedPullRequest, error)
	Upsert(
		ctx context.Context,
		p models.LinkedPullRequest,
	) error
	UpsertMany(
		ctx context.Context,
		ps []models.LinkedPullRequest,
	) error
}

// RoutineStore is the storage of scheduled routines.
type RoutineStore interface {
	Delete(
		ctx context.Context,
		name string,
	) (*models.Routine, error)
	Due(ctx context.Context) ([]models.Routine, error)
	Get(
		ctx context.Context,
		name string,
	) (*models.Routine, error)
	List(ctx context.Context) ([]models.Routine, error)
	Save(
		ctx context.Context,
		routine models.Routine,
	) (*models.Routine, error)
}

// ChatStore is the storage of chat sessions and their events.
type ChatStore interface {
	// Start pins the session to its runner and creates its running chat job; a
	// runner already at its job cap is a conflict.
	Start(
		ctx context.Context,
		c models.ChatSession,
		createdBy string,
	) (*models.ChatSession, error)
	Get(ctx context.Context, id uuid.UUID) (*models.ChatSession, error)
	List(
		ctx context.Context,
		activeOnly bool,
		limit int,
	) ([]models.ChatSession, error)
	// Transition moves the session from one of the states to another; a session
	// in none of them is a conflict.
	Transition(
		ctx context.Context,
		id uuid.UUID,
		from []models.ChatState,
		to models.ChatState,
	) (*models.ChatSession, error)
	// Append numbers and adds events to an unstopped session; a stopped one is a
	// conflict.
	Append(
		ctx context.Context,
		id uuid.UUID,
		events []models.ChatEventInput,
	) error
	After(
		ctx context.Context,
		id uuid.UUID,
		afterSeq, limit int,
	) ([]models.ChatEvent, error)
	// Stop ends an unstopped session, keeping its report and finishing its job
	// as jobStatus; a stopped one is a conflict.
	Stop(
		ctx context.Context,
		id uuid.UUID,
		report string,
		jobStatus models.JobStatus,
	) (*models.ChatSession, error)
	// ClaimFor marks the oldest unclaimed, unstopped session pinned to the runner
	// as claimed and returns it; none is not found.
	ClaimFor(ctx context.Context, runnerID uuid.UUID) (*models.ChatSession, error)
	// AppendReport adds text to a session's report, which a person's stop may have
	// set already.
	AppendReport(ctx context.Context, id uuid.UUID, text string) error
	// StopForJob stops the unstopped session of a chat job that ended without it;
	// none is not found.
	StopForJob(
		ctx context.Context,
		jobID uuid.UUID,
		report string,
	) (*models.ChatSession, error)
}

// Stores is the storage the services are built on.
type Stores struct {
	Tasks    TaskStore
	Repos    RepoStore
	Runners  RunnerStore
	Jobs     JobStore
	Events   JobEventStore
	Settings SettingsStore
	Linked   LinkedPullRequestStore
	Routines RoutineStore
	Chats    ChatStore
}
