package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

const (
	// lostRequeueAuthor creates the jobs that continue a lost job's branch.
	lostRequeueAuthor = "system:lost-requeue"
	// maxLostRequeues is how many times a task's lost job is requeued before the
	// task goes to Needs you.
	maxLostRequeues = 2
	// reconcileGrace is how long an In progress task may go untouched without a
	// job before the reconciler moves it: a person's own session has none.
	reconcileGrace = 30 * time.Minute
)

const lostRequeuePrompt = "The runner running this job was lost. Continue where " +
	"you left off on this branch, finish the work, and write the outcome."

// livePR reads where the job's pull request stands now, nil when it can't be told.
func (s *JobService) livePR(
	ctx context.Context,
	job *models.Job,
) *githubapp.PullRequestStatus {
	if s.github == nil || job == nil || job.PRURL == "" {
		return nil
	}
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return nil
	}
	number, ok := pullNumber(job.PRURL, repo)
	if !ok {
		return nil
	}
	status, err := s.github.PullRequest(ctx, repo.Owner, repo.Name, number)
	if err != nil {
		return nil
	}
	return &status
}

// settleLost settles the task of a lost job.
func (s *JobService) settleLost(
	ctx context.Context,
	job *models.Job,
	body string,
) error {
	task, err := s.tasks.Get(ctx, job.TaskID)
	if err != nil {
		return err
	}
	return s.settle(ctx, task, job, body)
}

// settle moves a task that has lost its job (last, nil when it has none) to where
// its work stands, commenting body: Testing for a merged pull request, In review
// for an open one, else back to work on the same branch, and Needs you only when
// there is nothing to resume.
func (s *JobService) settle(
	ctx context.Context,
	task *models.Task,
	last *models.Job,
	body string,
) error {
	state, url, sha := task.PRState, task.PRURL, task.MergeSHA
	if live := s.livePR(ctx, last); live != nil {
		state, url = live.State, last.PRURL
		if live.MergeSHA != "" {
			sha = live.MergeSHA
		}
	}
	switch state {
	case githubapp.StateMerged:
		return s.toTesting(ctx, task, url, sha, body)
	case githubapp.StateOpen:
		return s.finishTask(ctx, task.ID, systemAuthor,
			body+" Its pull request is open: "+url, models.StatusInReview)
	}
	requeued, err := s.requeue(ctx, task, last, body)
	if err != nil || requeued {
		return err
	}
	return s.finishTask(ctx, task.ID, systemAuthor, body, models.StatusNeedsYou)
}

func (s *JobService) toTesting(
	ctx context.Context,
	task *models.Task,
	url, sha, body string,
) error {
	if sha != "" && task.MergeSHA == "" {
		if err := s.tasks.SetMergeSHA(ctx, task.ID, sha); err != nil {
			return err
		}
	}
	return s.finishTask(ctx, task.ID, systemAuthor,
		body+" Its pull request was merged: "+url, models.StatusTesting)
}

// requeue queues a job that continues the task's newest job on its branch, in its
// session when it left one. It reports false when there is nothing to continue or
// the task was requeued too often.
func (s *JobService) requeue(
	ctx context.Context,
	task *models.Task,
	last *models.Job,
	body string,
) (bool, error) {
	if last == nil || last.IsRoutine() {
		return false, nil
	}
	jobs, err := s.jobs.ByTask(ctx, task.ID)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(jobs, func(j models.Job) bool {
		return !j.Status.Finished()
	}) {
		return true, nil
	}
	if n := countBy(jobs, lostRequeueAuthor); n >= maxLostRequeues {
		return false, nil
	}
	next := models.Job{
		ID: uuid.Nil, TaskID: task.ID, TaskNumber: 0, RepoID: last.RepoID,
		Harness: "", Model: "", Prompt: last.Prompt, Status: "",
		Branch: last.Branch, SessionID: "", Outcome: "",
		ParentJobID: last.ParentJobID, PRURL: "", RunnerID: nil, PRState: "",
		PRCI: "", PRReview: "", CreatedBy: lostRequeueAuthor, CreatedAt: time.Time{},
		Routine: "", Limited: false, LimitResetsAt: nil,
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	}
	if last.SessionID != "" {
		next.Prompt, next.ParentJobID = lostRequeuePrompt, &last.ID
	}
	if _, err = s.jobs.Create(ctx, next); err != nil {
		return false, err
	}
	err = s.finishTask(ctx, task.ID, systemAuthor,
		body+" Queued again on its branch.", models.StatusInProgress)
	return true, err
}

func countBy(jobs []models.Job, author string) int {
	n := 0
	for i := range jobs {
		if jobs[i].CreatedBy == author {
			n++
		}
	}
	return n
}

// ReconcileBoard moves the tasks the board shows wrongly: an In progress task
// with no job, untouched for a while, to where its work stands (a parked one to
// Backlog), and a Needs you task whose pull request merged to Testing. It returns
// how many moved; one failing task doesn't stop the others.
func (s *JobService) ReconcileBoard(ctx context.Context) (int, error) {
	all, err := s.tasks.List(ctx, "")
	if err != nil {
		return 0, err
	}
	moved := 0
	var failures []error
	for i := range all {
		t := &all[i]
		var did bool
		switch t.Status {
		case models.StatusInProgress:
			did, err = s.reconcileInProgress(ctx, t)
		case models.StatusNeedsYou:
			did, err = s.reconcileNeedsYou(ctx, t)
		case models.StatusBacklog, models.StatusReady, models.StatusInReview,
			models.StatusTesting, models.StatusDone:
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("task T-%d: %w", t.Number, err))
		}
		if did {
			moved++
		}
	}
	return moved, errors.Join(failures...)
}

func (s *JobService) reconcileInProgress(
	ctx context.Context,
	t *models.Task,
) (bool, error) {
	if s.now().Sub(t.UpdatedAt) < reconcileGrace {
		return false, nil
	}
	jobs, err := s.jobs.ByTask(ctx, t.ID)
	if err != nil || hasActive(jobs) {
		return false, err
	}
	if slices.Contains(t.Labels, ParkedLabel) {
		return true, s.finishTask(ctx, t.ID, systemAuthor,
			"Parked, not in progress: moved to Backlog until a new job image is "+
				"published.", models.StatusBacklog)
	}
	var last *models.Job
	if len(jobs) > 0 {
		last = &jobs[0]
	}
	return true, s.settle(ctx, t, last, "No job is running for this task.")
}

func (s *JobService) reconcileNeedsYou(
	ctx context.Context,
	t *models.Task,
) (bool, error) {
	if t.PRState != githubapp.StateMerged {
		return false, nil
	}
	jobs, err := s.jobs.ByTask(ctx, t.ID)
	if err != nil || hasActive(jobs) {
		return false, err
	}
	return true, s.toTesting(ctx, t, t.PRURL, "", "This task was waiting on you.")
}

func hasActive(jobs []models.Job) bool {
	return slices.ContainsFunc(jobs, func(j models.Job) bool {
		return !j.Status.Finished()
	})
}
