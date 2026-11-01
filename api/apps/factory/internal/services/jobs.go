package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

const (
	// LostAfter is how long a running job may stay silent before it is lost.
	LostAfter = 5 * time.Minute

	maxRunText = 10000
	// browserChannel is what a body edit from a browser session is stored as.
	browserChannel = "browser"
	maxSlugLen     = 30
	// systemAuthor signs the comments the factory writes itself.
	systemAuthor = "system:factory"
)

type JobService struct {
	jobs  *repositories.JobsRepository
	tasks *repositories.TasksRepository
	repos *repositories.ReposRepository
	now   func() time.Time
}

// ClaimedJob is a job with the repo it runs against.
type ClaimedJob struct {
	Job  models.Job
	Repo models.Repo
}

func conflict(msg string) error {
	return &app.HTTPError{Status: http.StatusConflict, Message: msg}
}

// slug lower-cases a title into [a-z0-9-], collapsing runs and trimming dashes.
func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := b.String()
	out = strings.TrimRight(out[:min(len(out), maxSlugLen)], "-")
	if out == "" {
		return "task"
	}
	return out
}

// branchFor is the agent/<task-id>-<slug> branch a task's jobs commit to.
func branchFor(task *models.Task) string {
	return "agent/" + task.ID.String() + "-" + slug(task.Title)
}

// promptFor freezes what the agent is told: the repo's instructions, the task as
// it stands, and the Run text. The runner adds its own preamble.
func promptFor(repo *models.Repo, task *models.Task, text string) string {
	var b strings.Builder
	if in := strings.TrimSpace(repo.AgentInstructions); in != "" {
		b.WriteString(in + "\n\n")
	}
	b.WriteString("# Task: " + task.Title + "\n\n" + strings.TrimSpace(task.Body))
	if text != "" {
		b.WriteString("\n\n# Instructions for this run\n\n" + text)
	}
	return b.String()
}

// Run queues a job for a task and moves the task to In progress. Only a human
// admin in a browser may run a task, as for Ready. A task whose body was last
// edited outside a browser session is refused, with a comment, and moved to
// Needs you: an agent could have put text there the owner never saw.
func (s *JobService) Run(
	ctx context.Context,
	actor models.Actor,
	taskID uuid.UUID,
	text string,
) (*models.Job, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can run a task")
	}
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > maxRunText {
		return nil, badRequest("run text is too long")
	}

	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.Get(ctx, task.RepoID)
	if err != nil {
		return nil, err
	}
	existing, err := s.jobs.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if !existing[i].Status.Finished() {
			return nil, conflict("this task already has a job queued or running")
		}
	}

	channel, err := s.tasks.BodyEditedChannel(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if channel != browserChannel {
		return nil, s.refuse(ctx, task)
	}

	job, err := s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, TaskID: taskID, RepoID: repo.ID, Harness: repo.DefaultHarness,
		Prompt: promptFor(repo, task, text), Status: "", Branch: branchFor(task),
		SessionID: "", Outcome: "", CreatedBy: actor.String(), CreatedAt: time.Time{},
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	})
	if err != nil {
		return nil, err
	}
	if _, err = s.tasks.Move(ctx, taskID, models.StatusInProgress); err != nil {
		return nil, err
	}
	return job, nil
}

// refuse comments why a task wasn't run and parks it in Needs you, once: a task
// already there has the comment, and another press must not repeat it.
func (s *JobService) refuse(ctx context.Context, task *models.Task) error {
	const reason = "Not run: the task's body was last edited outside a browser " +
		"session, so what an agent would be told isn't what you approved. Open the " +
		"task in the app and save its body, then run it again."
	if task.Status == models.StatusNeedsYou {
		return badRequest(reason)
	}
	if _, err := s.tasks.AddComment(ctx, task.ID, systemAuthor, reason); err != nil {
		return err
	}
	if _, err := s.tasks.Move(ctx, task.ID, models.StatusNeedsYou); err != nil {
		return err
	}
	return badRequest(reason)
}

// Claim starts the oldest queued job, or returns nil when there is none or one is
// already running. It first marks jobs that stopped checking in as lost.
func (s *JobService) Claim(ctx context.Context) (*ClaimedJob, error) {
	if err := s.ExpireLost(ctx); err != nil {
		return nil, err
	}
	job, err := s.jobs.Claim(ctx)
	if errors.Is(err, database.ErrResourceNotFound) ||
		errors.Is(err, database.ErrResourceConflict) {
		return nil, nil //nolint:nilnil // no job is an answer, not an error
	}
	if err != nil {
		return nil, err
	}
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return nil, err
	}
	return &ClaimedJob{Job: *job, Repo: *repo}, nil
}

// CheckIn records that the job is alive. It reports stop when the job is no
// longer running, so the runner abandons it.
func (s *JobService) CheckIn(
	ctx context.Context,
	id uuid.UUID,
	sessionID string,
) (bool, error) {
	err := s.jobs.CheckIn(ctx, id, sessionID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return true, nil
	}
	return false, err
}

// Complete finishes a running job, comments its outcome on the task and moves
// the task to Needs you: without a PR the branch stays in the Mac's clone.
func (s *JobService) Complete(
	ctx context.Context,
	runnerID uuid.UUID,
	id uuid.UUID,
	succeeded bool,
	outcome, sessionID string,
) (*models.Job, error) {
	status := models.JobFailed
	if succeeded {
		status = models.JobSucceeded
	}
	outcome = strings.TrimSpace(outcome)
	if utf8.RuneCountInString(outcome) > maxCommentLen {
		outcome = string([]rune(outcome)[:maxCommentLen])
	}
	job, err := s.jobs.Complete(ctx, id, status, outcome, sessionID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, conflict("the job is no longer running")
	}
	if err != nil {
		return nil, err
	}
	if outcome == "" {
		outcome = "(the agent wrote no outcome)"
	}
	body := "Job " + string(status) + " on branch `" + job.Branch + "`:\n\n" + outcome
	if err = s.finishTask(ctx, job.TaskID, "runner:"+runnerID.String(), body); err != nil {
		return nil, err
	}
	return job, nil
}

// ExpireLost marks running jobs that stopped checking in as lost and parks their
// tasks in Needs you.
func (s *JobService) ExpireLost(ctx context.Context) error {
	lost, err := s.jobs.ExpireLost(ctx, s.now().Add(-LostAfter))
	if err != nil {
		return err
	}
	for i := range lost {
		body := "Job lost: the runner stopped checking in. Its branch `" +
			lost[i].Branch + "` may hold partial work on the Mac."
		if err = s.finishTask(ctx, lost[i].TaskID, systemAuthor, body); err != nil {
			return err
		}
	}
	return nil
}

// SweepLost is ExpireLost for callers that can't act on a failure: reading the
// board must work even when the sweep doesn't, and the next read tries again.
func (s *JobService) SweepLost(ctx context.Context) {
	_ = s.ExpireLost(ctx)
}

func (s *JobService) finishTask(
	ctx context.Context,
	taskID uuid.UUID,
	author, body string,
) error {
	if _, err := s.tasks.AddComment(ctx, taskID, author, body); err != nil {
		return err
	}
	_, err := s.tasks.Move(ctx, taskID, models.StatusNeedsYou)
	return err
}

// ByTask lists a task's jobs, newest first.
func (s *JobService) ByTask(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Job, error) {
	return s.jobs.ByTask(ctx, taskID)
}
