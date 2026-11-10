package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/slack"
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
	jobs   *repositories.JobsRepository
	tasks  *repositories.TasksRepository
	repos  *repositories.ReposRepository
	events *repositories.JobEventsRepository
	// accounts is nil when jobs get no agent account of their own.
	accounts JobAccounts
	// github is nil when no GitHub App is set up.
	github   GitHub
	now      func() time.Time
	say      notices
	runners  *repositories.RunnersRepository
	settings *repositories.SettingsRepository
	linked   *repositories.LinkedPullRequestsRepository
	// alertedOffline is the queued job the offline-runner notice was last sent for,
	// so one waiting job is announced once.
	offlineMu      sync.Mutex
	alertedOffline uuid.UUID
}

// ClaimedJob is a job with the repo it runs against.
type ClaimedJob struct {
	Job  models.Job
	Repo models.Repo
	// MCPClientID and MCPClientSecret are the job's own agent account, when it has one.
	MCPClientID     string
	MCPClientSecret string
	// ResumeSessionID is the harness session to continue, for a Reply's job.
	ResumeSessionID string
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

// HarnessPick is a per-job choice of harness and model; the zero value keeps the
// repo's defaults.
type HarnessPick struct {
	Harness models.Harness
	Model   string
}

// resolveHarness is the harness and model a job runs on: the pick where it names
// one, else the repo's defaults. A pick of OpenCode without a model takes the repo's
// default model when the repo's default harness is OpenCode too.
func resolveHarness(
	repo *models.Repo,
	pick HarnessPick,
) (models.Harness, string, error) {
	harness := pick.Harness
	model := strings.TrimSpace(pick.Model)
	if harness == "" {
		harness = repo.DefaultHarness
		if model == "" {
			model = repo.DefaultModel
		}
	}
	if !harness.Valid() {
		return "", "", badRequest("invalid harness")
	}
	if harness == models.HarnessOpenCode && model == "" &&
		repo.DefaultHarness == models.HarnessOpenCode {
		model = repo.DefaultModel
	}
	if err := checkModel(harness, model); err != nil {
		return "", "", err
	}
	return harness, model, nil
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
	pick HarnessPick,
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
	harness, model, err := resolveHarness(repo, pick)
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
		ID: uuid.Nil, TaskID: taskID, RepoID: repo.ID, Harness: harness, Model: model,
		Prompt: promptFor(repo, task, text), Status: "", Branch: branchFor(task),
		SessionID: "", Outcome: "", ParentJobID: nil, PRURL: "", RunnerID: nil,
		PRState: "", PRCI: "", PRReview: "", CreatedBy: actor.String(),
		CreatedAt: time.Time{}, Limited: false, LimitResetsAt: nil,
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

// Reply queues a job that resumes the harness session of the task's newest job,
// on the same branch, with the reply as its prompt: "CI is red, fix it", or an
// answer to the agent's question. Only a human admin in a browser may reply, as
// for Run. The reply is typed there, so it needs no check of the task's body.
func (s *JobService) Reply(
	ctx context.Context,
	actor models.Actor,
	taskID uuid.UUID,
	text string,
) (*models.Job, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can reply")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, badRequest("a reply needs text")
	}
	if utf8.RuneCountInString(text) > maxRunText {
		return nil, badRequest("reply is too long")
	}
	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status == models.StatusDone {
		return nil, conflict("this task is done; its session is gone")
	}
	existing, err := s.jobs.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return nil, conflict("this task has no job to reply to")
	}
	for i := range existing {
		if !existing[i].Status.Finished() {
			return nil, conflict("this task already has a job queued or running")
		}
	}
	last := existing[0]
	if last.SessionID == "" {
		return nil, conflict("the last job left no session to resume")
	}
	job, err := s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, TaskID: taskID, RepoID: last.RepoID, Harness: last.Harness,
		Model:  last.Model,
		Prompt: text, Status: "", Branch: last.Branch, SessionID: "", Outcome: "",
		ParentJobID: &last.ID, PRURL: "", RunnerID: nil, PRState: "", PRCI: "",
		PRReview: "", CreatedBy: actor.String(), CreatedAt: time.Time{},
		Limited: false, LimitResetsAt: nil,
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

// FinishedTasks returns those of the given tasks that are Done or no longer exist:
// the runner's session state for them can go.
func (s *JobService) FinishedTasks(
	ctx context.Context,
	ids []uuid.UUID,
) ([]uuid.UUID, error) {
	statuses, err := s.tasks.Statuses(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := []uuid.UUID{}
	for _, id := range ids {
		if status, ok := statuses[id]; !ok || status == models.StatusDone {
			out = append(out, id)
		}
	}
	return out, nil
}

// Stop cancels the task's queued or running job and parks the task in Needs you.
// A running job's runner learns of it on its next check-in, which no longer finds
// the job running. Only a human admin in a browser may stop, as for Run.
func (s *JobService) Stop(
	ctx context.Context,
	actor models.Actor,
	taskID uuid.UUID,
) (*models.Job, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can stop a job")
	}
	if _, err := s.tasks.Get(ctx, taskID); err != nil {
		return nil, err
	}
	job, err := s.jobs.CancelForTask(ctx, taskID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, conflict("this task has no queued or running job")
	}
	if err != nil {
		return nil, err
	}
	s.revokeAccount(ctx, job.ID)
	body := "Job stopped by " + actor.String() + "."
	if job.StartedAt != nil {
		body += " Its branch `" + job.Branch + "` may hold partial work on the Mac."
	}
	if err = s.finishTask(
		ctx, taskID, systemAuthor, body, models.StatusNeedsYou,
	); err != nil {
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
func (s *JobService) Claim(
	ctx context.Context,
	runnerID uuid.UUID,
) (*ClaimedJob, error) {
	if err := s.ExpireLost(ctx); err != nil {
		return nil, err
	}
	// Best effort: a failure here leaves the queue as it was, and the next claim
	// looks again.
	_ = s.pickUp(ctx)
	job, err := s.jobs.Claim(ctx, runnerID)
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
	resume := ""
	if job.ParentJobID != nil {
		parent, parentErr := s.jobs.Get(ctx, *job.ParentJobID)
		if parentErr != nil {
			return nil, parentErr
		}
		resume = parent.SessionID
	}
	claimed := &ClaimedJob{Job: *job, Repo: *repo, MCPClientID: "", MCPClientSecret: "",
		ResumeSessionID: resume}
	if s.accounts != nil {
		if err = s.issueAccount(ctx, claimed, runnerID); err != nil {
			return nil, err
		}
	}
	return claimed, nil
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

// Complete finishes a running job and comments its outcome on the task. A
// succeeded job that opened a pull request of its own repo moves the task to In
// review, with the link; any other job moves it to Needs you, because the branch
// is then only in the Mac's clone. A pull request link is kept for a failed job
// too, so an open PR is never orphaned. Only the runner that claimed the job may
// complete it.
func (s *JobService) Complete(
	ctx context.Context,
	runnerID uuid.UUID,
	id uuid.UUID,
	succeeded bool,
	outcome, sessionID, prURL string,
	limited bool,
	limitResetsAt *time.Time,
) (*models.Job, error) {
	// A job a usage limit stopped did not succeed, whatever the runner says.
	succeeded = succeeded && !limited
	status := models.JobFailed
	if succeeded {
		status = models.JobSucceeded
	}
	outcome = strings.TrimSpace(outcome)
	if utf8.RuneCountInString(outcome) > maxCommentLen {
		outcome = string([]rune(outcome)[:maxCommentLen])
	}
	prURL, note := s.checkedPullRequest(ctx, id, strings.TrimSpace(prURL))
	resetsAt := plausibleReset(limitResetsAt, time.Now())
	job, err := s.jobs.Complete(ctx, id, status, outcome, sessionID, prURL, runnerID,
		limited, resetsAt)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, conflict("the job is no longer running")
	}
	if err != nil {
		return nil, err
	}
	s.revokeAccount(ctx, job.ID)
	if outcome == "" {
		outcome = "(the agent wrote no outcome)"
	}
	body := "Job " + string(status) + " on branch `" + job.Branch + "`"
	if limited {
		body = "Job stopped by Claude's usage limit on branch `" + job.Branch + "`"
	}
	next := models.StatusNeedsYou
	if prURL != "" {
		body += ", pull request: " + prURL
		if succeeded {
			next = models.StatusInReview
		}
	}
	body += ":\n\n" + outcome + note
	author := "runner:" + runnerID.String()
	if err = s.finishTask(ctx, job.TaskID, author, body, next); err != nil {
		return nil, err
	}
	s.announceEnd(ctx, job, succeeded, prURL, outcome)
	return job, nil
}

// maxLimitAhead bounds a reset time worth keeping: weekly caps are the longest.
const maxLimitAhead = 15 * 24 * time.Hour

// plausibleReset is a runner's reset time if it is in the coming fifteen days, else
// nil: the header must not show a nonsense time.
func plausibleReset(at *time.Time, now time.Time) *time.Time {
	if at == nil || at.Before(now.Add(-time.Minute)) || at.After(now.Add(maxLimitAhead)) {
		return nil
	}
	utc := at.UTC()
	return &utc
}

// limitUntil is " (until <time>)" for a known reset, else nothing.
func limitUntil(at *time.Time) string {
	if at == nil {
		return ""
	}
	return " (until " + at.UTC().Format("2006-01-02 15:04 MST") + ")"
}

// ActiveLimit is when the latest Claude usage limit lifts, nil if none is in force.
func (s *JobService) ActiveLimit(ctx context.Context) (*time.Time, error) {
	return s.jobs.ActiveLimit(ctx)
}

// announceEnd tells Slack how a job ended: a pull request to review, or a task
// that needs the owner. Only the task's title, a link and the outcome's first line
// leave; none of the job's log or diff does.
func (s *JobService) announceEnd(
	ctx context.Context,
	job *models.Job,
	succeeded bool,
	prURL, outcome string,
) {
	link := s.taskLink(ctx, job.TaskID)
	switch {
	case job.Limited:
		s.say.say(ctx, ":hourglass_flowing_sand: Claude's usage limit stopped a job on "+
			link+limitUntil(job.LimitResetsAt))
	case succeeded && prURL != "":
		s.say.say(ctx, ":white_check_mark: Pull request opened for "+link+": <"+
			prURL+"|review it>")
	case succeeded:
		s.say.say(ctx, ":warning: A job finished with no pull request for "+link+
			", which needs you: "+slack.Escape(firstLine(outcome)))
	default:
		s.say.say(ctx, ":x: A job failed on "+link+": "+slack.Escape(firstLine(outcome)))
	}
}

// taskLink is the task as a Slack link, or plain words when it can't be read.
func (s *JobService) taskLink(ctx context.Context, taskID uuid.UUID) string {
	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return "a task"
	}
	return s.say.taskLink(task.ID, task.Title)
}

// checkedPullRequest keeps a reported pull request only if it is the job repo's
// own. A link it drops is mentioned on the task, and a failure to check never
// stops the job from completing: the link is dropped with a note instead.
func (s *JobService) checkedPullRequest(
	ctx context.Context,
	jobID uuid.UUID,
	prURL string,
) (string, string) {
	if prURL == "" {
		return "", ""
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return "", noteDropped("it could not be checked")
	}
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return "", noteDropped("it could not be checked")
	}
	if !ownPullRequest(prURL, repo) {
		return "", noteDropped("it is not a pull request of this repo")
	}
	return prURL, ""
}

func noteDropped(why string) string {
	return "\n\n(The runner reported a pull request link, left out because " + why + ".)"
}

// ExpireLost marks running jobs that stopped checking in as lost and parks their
// tasks in Needs you.
func (s *JobService) ExpireLost(ctx context.Context) error {
	// Whatever else fails, no ended job keeps a working secret.
	defer s.sweepAccounts(ctx)
	lost, err := s.jobs.ExpireLost(ctx, s.now().Add(-LostAfter))
	if err != nil {
		return err
	}
	for i := range lost {
		body := "Job lost: the runner stopped checking in. Its branch `" +
			lost[i].Branch + "` may hold partial work on the Mac."
		if err = s.finishTask(
			ctx, lost[i].TaskID, systemAuthor, body, models.StatusNeedsYou,
		); err != nil {
			return err
		}
		s.say.say(ctx, ":x: A job was lost on "+s.taskLink(ctx, lost[i].TaskID)+
			": the runner stopped checking in.")
	}
	return nil
}

// autoPickupAuthor is who a job queued by auto pickup is attributed to.
const autoPickupAuthor = "system:auto-pickup"

// pickUp queues a job for the oldest highest-priority Ready task of a repo with auto
// pickup on, when nothing is queued or running and the board isn't paused. The
// Ready move was the approval, so the prompt is built from the text as it stood
// then, not the task's current text. The runner calling Claim is the online runner
// the job is for.
func (s *JobService) pickUp(ctx context.Context) error {
	next, err := s.tasks.NextReady(ctx)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	repo, err := s.repos.Get(ctx, next.RepoID)
	if err != nil {
		return err
	}
	harness, model, err := resolveHarness(repo, HarnessPick{Harness: "", Model: ""})
	if err != nil {
		return s.cannotPickUp(ctx, next.TaskID, err)
	}
	// While Claude is limited a Claude job would only fail the same way, and each
	// one would park its task in Needs you. The oldest Ready task waits.
	if harness == models.HarnessClaudeCode {
		if until, limitErr := s.jobs.ActiveLimit(ctx); limitErr == nil && until != nil {
			return nil
		}
	}
	approved := approvedTask(next)
	_, err = s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, TaskID: next.TaskID, RepoID: repo.ID, Harness: harness,
		Model: model, Prompt: promptFor(repo, approved, ""), Status: "",
		Branch: branchFor(approved), SessionID: "", Outcome: "", ParentJobID: nil,
		PRURL: "", RunnerID: nil, PRState: "", PRCI: "", PRReview: "",
		CreatedBy: autoPickupAuthor, CreatedAt: time.Time{}, StartedAt: nil,
		Limited: false, LimitResetsAt: nil, LastCheckIn: nil, FinishedAt: nil,
	})
	if err != nil {
		return err
	}
	_, err = s.tasks.Move(ctx, next.TaskID, models.StatusInProgress)
	return err
}

// cannotPickUp records why a Ready task the repo's settings can't run is passed
// over, so one such task doesn't hold up every other and the owner can see why.
// It leaves the task in Ready (the factory skipped it; it isn't waiting on you);
// a fresh Ready move clears the reason and retries.
func (s *JobService) cannotPickUp(
	ctx context.Context,
	taskID uuid.UUID,
	reason error,
) error {
	body := "Not picked up automatically: " + reason.Error() +
		". Fix the repo's settings, then move the task to Ready again to retry."
	if _, err := s.tasks.AddComment(ctx, taskID, systemAuthor, body); err != nil {
		return err
	}
	_, err := s.tasks.BlockPickup(ctx, taskID, reason.Error())
	return err
}

// approvedTask is the task as a human approved it at the Ready move: only the text
// a prompt and a branch name are built from.
func approvedTask(c *repositories.PickupCandidate) *models.Task {
	return &models.Task{
		ID: c.TaskID, RepoID: c.RepoID, Title: c.Title, Body: c.Body, Status: "",
		Priority: "", Labels: nil, CreatedBy: "", CreatedAt: time.Time{},
		UpdatedAt: time.Time{}, CommentCount: 0, PRURL: "", PRState: "", PRCI: "",
		PRReview: "", PickupApproved: true, PickupBlocked: "",
	}
}

// PickupPaused reports whether auto pickup is paused for the whole board.
func (s *JobService) PickupPaused(ctx context.Context) (bool, error) {
	return s.settings.PickupPaused(ctx)
}

// SetPickupPaused pauses or resumes auto pickup. Pausing is the emergency stop, so
// anyone who can reach the board may. Resuming lets agents start work unattended
// again, so like the Ready move it takes a human admin in a browser.
func (s *JobService) SetPickupPaused(
	ctx context.Context,
	actor models.Actor,
	paused bool,
) error {
	if !paused && !actor.CanMoveToReady() {
		return forbidden("only a human admin in a browser session can resume pickup")
	}
	return s.settings.SetPickupPaused(ctx, paused)
}

// OfflineAfter is how long a job may wait with no runner online before Slack hears
// of it.
const OfflineAfter = 10 * time.Minute

// CheckRunnerOffline tells Slack once when the oldest queued job has waited longer
// than OfflineAfter and no runner is online. A runner that is online but busy
// with another job is not offline.
func (s *JobService) CheckRunnerOffline(ctx context.Context) error {
	job, err := s.jobs.OldestQueued(ctx)
	if errors.Is(err, database.ErrResourceNotFound) {
		s.setAlerted(uuid.Nil)
		return nil
	}
	if err != nil {
		return err
	}
	now := s.now()
	waited := now.Sub(job.CreatedAt)
	if waited < OfflineAfter {
		return nil
	}
	runners, err := s.runners.List(ctx)
	if err != nil {
		return err
	}
	for i := range runners {
		if runners[i].Online(now) {
			return nil
		}
	}
	if !s.setAlerted(job.ID) {
		return nil
	}
	s.say.say(ctx, fmt.Sprintf(
		":zzz: No runner is online, and %s has waited %d minutes for one.",
		s.taskLink(ctx, job.TaskID), int(waited.Minutes())))
	return nil
}

// setAlerted records the job the offline notice is for and reports whether that
// is a change, so the caller sends it once.
func (s *JobService) setAlerted(id uuid.UUID) bool {
	s.offlineMu.Lock()
	defer s.offlineMu.Unlock()
	changed := s.alertedOffline != id
	s.alertedOffline = id
	return changed
}

// SweepLost is ExpireLost for callers that can't act on a failure: reading the
// board must work even when the sweep doesn't, and the next read tries again.
func (s *JobService) SweepLost(ctx context.Context) {
	_ = s.ExpireLost(ctx)
}

// finishTask comments on the task and moves it to the column the job leaves it in.
func (s *JobService) finishTask(
	ctx context.Context,
	taskID uuid.UUID,
	author, body string,
	to models.Status,
) error {
	if _, err := s.tasks.AddComment(ctx, taskID, author, body); err != nil {
		return err
	}
	_, err := s.tasks.Move(ctx, taskID, to)
	return err
}

// ByTask lists a task's jobs, newest first.
func (s *JobService) ByTask(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Job, error) {
	return s.jobs.ByTask(ctx, taskID)
}
