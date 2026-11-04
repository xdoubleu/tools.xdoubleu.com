package services

import (
	"context"
	"errors"
	"fmt"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/githubapp"
)

// pollBatch is how many open pull requests one poll reads, the longest unread
// first: a batch that is too big for GitHub's rate limit would starve the rest.
const pollBatch = 50

// PollPullRequests reads where each open pull request of a job stands, keeps it
// on the job for the board, and moves the task when the pull request ends: a
// merge sends a task that is In review to Done, and a close without a merge back
// to Needs you. It does nothing without a GitHub App. One failing pull request
// doesn't stop the others; the errors come back together.
func (s *JobService) PollPullRequests(ctx context.Context) (int, error) {
	if s.github == nil {
		return 0, nil
	}
	jobs, err := s.jobs.WithOpenPullRequest(ctx, pollBatch)
	if err != nil {
		return 0, err
	}
	var failures []error
	for i := range jobs {
		if err = s.pollPullRequest(ctx, &jobs[i]); err != nil {
			failures = append(failures, fmt.Errorf("job %s: %w", jobs[i].ID, err))
			// Take its turn at the back: a few that always fail must not starve the
			// healthy ones of every batch.
			_ = s.jobs.MarkPullRequestPolled(ctx, jobs[i].ID)
		}
	}
	return len(jobs), errors.Join(failures...)
}

func (s *JobService) pollPullRequest(ctx context.Context, job *models.Job) error {
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return err
	}
	number, ok := pullNumber(job.PRURL, repo)
	if !ok {
		return errors.New("its pull request address is not this repo's")
	}
	// A pull request can't be deleted, only closed, so "not found" means the App can
	// no longer see it (a rename, a transfer, changed access): that is a failure to
	// look into, never a reason to call the pull request closed.
	status, err := s.github.PullRequest(ctx, repo.Owner, repo.Name, number)
	if err != nil {
		return err
	}
	if status.State != job.PRState && status.State != githubapp.StateOpen {
		// First the task, then the record: a failure in between repeats this step
		// on the next poll, and the task's own status keeps it from happening twice.
		if err = s.pullRequestEnded(ctx, job, status.State); err != nil {
			return err
		}
	}
	return s.jobs.SetPullRequestStatus(ctx, job.ID, status.State, status.CI, status.Review)
}

// pullRequestEnded moves the job's task, but only if it is still waiting in In
// review and this is its newest job: a task a person has moved on, or one a later
// job is working on, is not ours to move.
func (s *JobService) pullRequestEnded(
	ctx context.Context,
	job *models.Job,
	state string,
) error {
	task, err := s.tasks.Get(ctx, job.TaskID)
	if err != nil {
		return err
	}
	if task.Status != models.StatusInReview {
		return nil
	}
	jobs, err := s.jobs.ByTask(ctx, job.TaskID)
	if err != nil {
		return err
	}
	if len(jobs) > 0 && jobs[0].ID != job.ID {
		return nil
	}
	to, body := models.StatusNeedsYou, "The pull request was closed without merging: "
	if state == githubapp.StateMerged {
		to, body = models.StatusDone, "The pull request was merged: "
	}
	// Move first: a failure after it loses a comment, where a failure before it
	// would repeat the comment on every poll until the move worked.
	if _, err = s.tasks.Move(ctx, job.TaskID, to); err != nil {
		return err
	}
	_, err = s.tasks.AddComment(ctx, job.TaskID, systemAuthor, body+job.PRURL)
	return err
}
