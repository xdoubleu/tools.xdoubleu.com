package services

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// StatusNames lists the board columns as the agent tools name them.
const StatusNames = "backlog, ready, in_progress, in_review, needs_you, testing or done"

// MaxBulkMove bounds one MoveTasks call.
const MaxBulkMove = 200

// shortJobID is how many characters of a job id name it in a notice.
const shortJobID = 8

// BoardService is what a person in the browser and an agent over MCP both do to
// tasks and routines: one rule set, whoever the caller is.
type BoardService struct {
	tasks *TaskService
	jobs  *JobService
}

// Board is the task list with the hints the board shows beside it.
type Board struct {
	Tasks        []models.Task
	PickupPaused bool
	// LimitedUntil is nil when the model is not rate limited.
	LimitedUntil *time.Time
	QueueStalled bool
}

// TaskDetail is a task with everything shown beside it.
type TaskDetail struct {
	Task     *models.Task
	Comments []models.Comment
	Jobs     []models.Job
	Pulls    []models.LinkedPullRequest
}

// TaskPatch is a partial edit: a nil field is left as it is. Labels replaces them
// all; the removals (ignoring case) and then the additions apply on top.
type TaskPatch struct {
	Title        *string
	Body         *string
	Priority     *models.Priority
	Labels       *[]string
	AddLabels    []string
	RemoveLabels []string
}

// BulkFailure is a task a bulk move could not move, and why.
type BulkFailure struct {
	ID    string
	Error string
}

// BulkMove is the outcome of moving many tasks.
type BulkMove struct {
	Moved  []string
	Failed []BulkFailure
}

// scope is the task a job's own account is confined to. A routine job's account
// is scoped but has no task, so it reads, files and comments board-wide while
// still unable to move or edit tasks.
type scope struct {
	scoped bool
	task   uuid.UUID
}

func unscoped() scope {
	return scope{scoped: false, task: uuid.Nil}
}

func (c scope) confined() bool {
	return c.scoped && c.task != uuid.Nil
}

// scopeOf is the limit on the actor's account. Only an agent's account can have
// one.
func (b *BoardService) scopeOf(ctx context.Context, actor models.Actor) (scope, error) {
	if actor.Kind != models.ActorAgent {
		return unscoped(), nil
	}
	userID, err := uuid.Parse(actor.ID)
	if err != nil {
		return unscoped(), forbidden("this tool is for agent accounts only")
	}
	id, err := b.jobs.TaskScope(ctx, userID)
	if err != nil || id == nil {
		return unscoped(), err
	}
	return scope{scoped: true, task: *id}, nil
}

// refuseJob stops a job's own account from doing what only a board-wide account
// may.
func (b *BoardService) refuseJob(
	ctx context.Context,
	actor models.Actor,
	what string,
) error {
	sc, err := b.scopeOf(ctx, actor)
	if err != nil {
		return err
	}
	if sc.scoped {
		return forbidden("a job can't " + what)
	}
	return nil
}

// onlyOwnTask hides a task other than the account's own, if it has a scope: a
// job's account hears the same as for one that doesn't exist. Sandbox tasks stay
// visible, as that is where it reports a gap.
func (b *BoardService) onlyOwnTask(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
) error {
	sc, err := b.scopeOf(ctx, actor)
	if err != nil {
		return err
	}
	if sc.confined() && id != sc.task {
		task, _, getErr := b.tasks.Get(ctx, id)
		if getErr != nil || !IsSandboxTask(task) {
			return database.ErrResourceNotFound
		}
	}
	return nil
}

// ListTasks lists the tasks the filter keeps and the actor may see, without their
// bodies. The board is polled while a runner may be gone, so this is where a job
// that stopped checking in gets noticed and its task parked in Needs you.
func (b *BoardService) ListTasks(
	ctx context.Context,
	actor models.Actor,
	f models.TaskFilter,
) (*Board, error) {
	if f.Status != "" && !f.Status.Valid() {
		return nil, badRequest("status must be one of " + StatusNames)
	}
	sc, err := b.scopeOf(ctx, actor)
	if err != nil {
		return nil, err
	}
	b.jobs.SweepLost(ctx)
	tasks, err := b.tasks.Find(ctx, f)
	if err != nil {
		return nil, err
	}
	if sc.confined() {
		tasks = slices.DeleteFunc(tasks, func(t models.Task) bool {
			return t.ID != sc.task && !IsSandboxTask(&t)
		})
	}
	b.jobs.FillDeploy(ctx, tasks)
	paused, err := b.jobs.PickupPaused(ctx)
	if err != nil {
		return nil, err
	}
	// The hints below are optional: if one cannot be read, the board still loads.
	until, _ := b.jobs.ActiveLimit(ctx)
	stalled, _ := b.jobs.Stalled(ctx)
	return &Board{
		Tasks: tasks, PickupPaused: paused, LimitedUntil: until, QueueStalled: stalled,
	}, nil
}

// GetTask returns one task by reference with its comments, jobs and pull requests.
func (b *BoardService) GetTask(
	ctx context.Context,
	actor models.Actor,
	ref string,
) (*TaskDetail, error) {
	id, err := b.tasks.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err = b.onlyOwnTask(ctx, actor, id); err != nil {
		return nil, err
	}
	// The sheet polls this while a job runs, so a dead runner's job must be
	// marked lost here too.
	b.jobs.SweepLost(ctx)
	task, comments, err := b.tasks.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	filled := []models.Task{*task}
	b.jobs.FillDeploy(ctx, filled)
	jobs, err := b.jobs.ByTask(ctx, id)
	if err != nil {
		return nil, err
	}
	pulls, err := b.jobs.PullRequestsOf(ctx, id)
	if err != nil {
		return nil, err
	}
	return &TaskDetail{Task: &filled[0], Comments: comments, Jobs: jobs, Pulls: pulls}, nil
}

// GetJob returns a job with its last events; a job's own account reads only its
// own task's jobs, as for the task itself.
func (b *BoardService) GetJob(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	tail int,
) (*models.Job, []models.JobEvent, error) {
	if tail <= 0 {
		tail = DefaultTailEvents
	}
	job, events, err := b.jobs.TailEvents(ctx, id, tail)
	if err != nil {
		return nil, nil, err
	}
	if err = b.onlyOwnTask(ctx, actor, job.TaskID); err != nil {
		return nil, nil, err
	}
	return job, events, nil
}

// CreateTask files a task. A job's account may only report a sandbox problem: the
// label, priority and column are fixed, and a repeat comments on the open task.
func (b *BoardService) CreateTask(
	ctx context.Context,
	actor models.Actor,
	repoID uuid.UUID,
	in TaskInput,
) (*models.Task, error) {
	sc, err := b.scopeOf(ctx, actor)
	if err != nil {
		return nil, err
	}
	if sc.confined() {
		task, _, repErr := b.tasks.ReportSandbox(ctx, actor, repoID, in.Title, in.Body)
		return task, repErr
	}
	return b.tasks.Create(ctx, actor, repoID, in)
}

// AddComment comments on the task a reference names.
func (b *BoardService) AddComment(
	ctx context.Context,
	actor models.Actor,
	ref, body string,
) (*models.Comment, error) {
	id, err := b.tasks.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err = b.onlyOwnTask(ctx, actor, id); err != nil {
		return nil, err
	}
	return b.tasks.Comment(ctx, actor, id, body)
}

// MoveTask moves the task a reference names to a column. A job's account can't:
// the factory moves its task when the job ends.
func (b *BoardService) MoveTask(
	ctx context.Context,
	actor models.Actor,
	ref string,
	status models.Status,
) (*models.Task, error) {
	id, err := b.tasks.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err = b.refuseJobMove(ctx, actor); err != nil {
		return nil, err
	}
	return b.tasks.Move(ctx, actor, id, status)
}

func (b *BoardService) refuseJobMove(ctx context.Context, actor models.Actor) error {
	sc, err := b.scopeOf(ctx, actor)
	if err != nil {
		return err
	}
	if sc.scoped {
		return forbidden(
			"a job can't move tasks: the factory moves its task when the job ends")
	}
	return nil
}

// MoveTasks moves many tasks to one column and reports which could not move.
func (b *BoardService) MoveTasks(
	ctx context.Context,
	actor models.Actor,
	refs []string,
	status models.Status,
) (*BulkMove, error) {
	if err := b.refuseJob(ctx, actor, "move tasks"); err != nil {
		return nil, err
	}
	refs, err := checkBulk(refs, status)
	if err != nil {
		return nil, err
	}
	out := &BulkMove{Moved: make([]string, 0, len(refs)), Failed: []BulkFailure{}}
	for _, ref := range refs {
		id, moveErr := b.tasks.Resolve(ctx, ref)
		if moveErr == nil {
			_, moveErr = b.tasks.Move(ctx, actor, id, status)
		}
		if moveErr != nil {
			out.Failed = append(out.Failed, BulkFailure{ID: ref, Error: moveErr.Error()})
			continue
		}
		out.Moved = append(out.Moved, ref)
	}
	return out, nil
}

// checkBulk validates a bulk move and returns its references sorted and deduped.
func checkBulk(refs []string, status models.Status) ([]string, error) {
	if len(refs) == 0 || len(refs) > MaxBulkMove {
		return nil, badRequest("ids must hold between 1 and 200 task ids")
	}
	if !status.Valid() {
		return nil, badRequest("status must be one of " + StatusNames)
	}
	return slices.Compact(slices.Sorted(slices.Values(refs))), nil
}

// UpdateTask applies a patch to the task a reference names. A job's account
// can't edit. Any other actor may edit a task in any column: a Ready task's
// approved text is snapshotted at the Ready move, so a running job is unaffected.
func (b *BoardService) UpdateTask(
	ctx context.Context,
	actor models.Actor,
	ref string,
	patch TaskPatch,
) (*models.Task, error) {
	if err := b.refuseJob(ctx, actor, "edit tasks"); err != nil {
		return nil, err
	}
	id, err := b.tasks.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	current, _, err := b.tasks.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	in := applyPatch(current, patch)
	return b.tasks.Update(ctx, actor, id, in)
}

// applyPatch is the edit the patch makes to the current task.
func applyPatch(current *models.Task, patch TaskPatch) TaskInput {
	in := TaskInput{
		Title: current.Title, Body: current.Body, Priority: current.Priority,
		Labels: current.Labels,
	}
	if patch.Title != nil {
		in.Title = *patch.Title
	}
	if patch.Body != nil {
		in.Body = *patch.Body
	}
	if patch.Priority != nil {
		in.Priority = *patch.Priority
	}
	if patch.Labels != nil {
		in.Labels = *patch.Labels
	}
	in.Labels = mergeLabels(in.Labels, patch.AddLabels, patch.RemoveLabels)
	return in
}

// mergeLabels applies the removals (ignoring case) and then the additions,
// keeping order and dropping duplicates.
func mergeLabels(current, add, remove []string) []string {
	out := []string{}
	for _, l := range current {
		if !containsFold(remove, l) {
			out = append(out, l)
		}
	}
	for _, l := range add {
		if !containsFold(out, l) {
			out = append(out, l)
		}
	}
	return out
}

func containsFold(list []string, label string) bool {
	return slices.ContainsFunc(list, func(l string) bool {
		return strings.EqualFold(l, label)
	})
}

// ListRoutines lists every routine with its newest job.
func (b *BoardService) ListRoutines(ctx context.Context) ([]models.Routine, error) {
	return b.jobs.ListRoutines(ctx)
}

// SaveRoutine creates or changes a routine.
func (b *BoardService) SaveRoutine(
	ctx context.Context,
	actor models.Actor,
	in RoutineInput,
) (*models.Routine, error) {
	actor, err := b.routineActor(ctx, actor, in.Name)
	if err != nil {
		return nil, err
	}
	return b.jobs.SaveRoutine(ctx, actor, in)
}

// DeleteRoutine deletes a routine and cancels its queued or running job.
func (b *BoardService) DeleteRoutine(
	ctx context.Context,
	actor models.Actor,
	name string,
) error {
	actor, err := b.routineActor(ctx, actor, name)
	if err != nil {
		return err
	}
	return b.jobs.DeleteRoutine(ctx, actor, name)
}

// RunRoutine queues a job for a routine now.
func (b *BoardService) RunRoutine(
	ctx context.Context,
	actor models.Actor,
	name string,
) (*models.Job, error) {
	actor, err := b.routineActor(ctx, actor, name)
	if err != nil {
		return nil, err
	}
	return b.jobs.RunRoutine(ctx, actor, name)
}

// routineActor is the actor as allowed to manage routines. A job's account may,
// but not the routine its own job runs for, so a routine can't re-run or
// reconfigure itself. Its notices name the job and its task.
func (b *BoardService) routineActor(
	ctx context.Context,
	actor models.Actor,
	routine string,
) (models.Actor, error) {
	if actor.Kind != models.ActorAgent {
		return actor, nil
	}
	userID, err := uuid.Parse(actor.ID)
	if err != nil {
		return actor, forbidden("this tool is for agent accounts only")
	}
	job, err := b.jobs.JobOf(ctx, userID)
	if err != nil {
		return actor, err
	}
	if job == nil {
		return actor, nil
	}
	if job.Routine != "" && job.Routine == routine {
		return actor, forbidden("a job can't run or change the routine it runs for")
	}
	actor.Via = "job " + job.ID.String()[:shortJobID]
	if job.TaskNumber > 0 {
		actor.Via += " on T-" + strconv.FormatInt(job.TaskNumber, 10)
	}
	return actor, nil
}
