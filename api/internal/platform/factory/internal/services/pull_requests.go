package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/githubapp"
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
	moved, err := s.mergedLinked(ctx)
	if err != nil {
		failures = append(failures, err)
	}
	if err = s.syncLinked(ctx); err != nil {
		failures = append(failures, err)
	}
	return len(jobs) + moved, errors.Join(failures...)
}

// taskLine is the `Task: <title> (<id>)` line a pull request body links its task
// with.
// The id is T-<number>, or a UUID in pull requests opened before numbers.
var taskLine = regexp.MustCompile(
	`(?m)^Task: .*\((T-[0-9]+|[0-9a-fA-F-]{36})\)\s*$`)

// mergedLinked moves to Testing the task of each recently merged pull request
// that links one, whoever opened it. It leaves a task that has jobs (their own
// pull requests are polled above), one a person has moved on, and one that is
// not In review. A repeat does nothing, as the task has moved. The App's webhook
// does the same per merge; this catches a delivery that never arrived.
func (s *JobService) mergedLinked(ctx context.Context) (int, error) {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return 0, err
	}
	var failures []error
	moved := 0
	for i := range repos {
		pulls, listErr := s.github.MergedPullRequests(ctx, repos[i].Owner, repos[i].Name)
		if listErr != nil {
			failures = append(failures, fmt.Errorf("repo %s: %w", repos[i].Name, listErr))
			continue
		}
		for _, p := range pulls {
			done, moveErr := s.TestingAfterMerge(ctx, p.Body, p.URL, p.MergeSHA)
			if moveErr != nil {
				failures = append(failures, fmt.Errorf("pull request %s: %w", p.URL, moveErr))
			}
			if done {
				moved++
			}
		}
	}
	return moved, errors.Join(failures...)
}

// linkedTask is the reference (T-<number> or UUID) a pull request body links.
func linkedTask(body string) (string, bool) {
	m := taskLine.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// TestingAfterMerge moves to Testing the task a merged pull request links in its
// body, under the same rules as the poll, and keeps the merge commit for the
// deploy state. It reports whether a task moved.
func (s *JobService) TestingAfterMerge(
	ctx context.Context,
	prBody, prURL, mergeSHA string,
) (bool, error) {
	ref, ok := linkedTask(prBody)
	if !ok {
		return false, nil
	}
	id, err := s.tasks.Resolve(ctx, ref)
	if err != nil {
		// A link to a task that is gone is not a failure.
		return false, nil //nolint:nilerr // nothing to move
	}
	task, err := s.tasks.Get(ctx, id)
	if err != nil {
		return false, nil //nolint:nilerr // nothing to move
	}
	// Also backfills a task that merged without its commit being recorded.
	if task.MergeSHA == "" && mergeSHA != "" {
		err = s.tasks.SetMergeSHA(ctx, id, mergeSHA)
	}
	if err == nil {
		err = s.linked.Upsert(ctx, models.LinkedPullRequest{
			URL: prURL, TaskID: id, State: githubapp.StateMerged,
			CI: "", Review: "", CheckedAt: time.Time{},
		})
	}
	if err != nil {
		return false, err
	}
	if task.Status != models.StatusInReview {
		return false, nil
	}
	jobs, err := s.jobs.ByTask(ctx, id)
	if err != nil {
		return false, err
	}
	if len(jobs) > 0 {
		return false, nil
	}
	if _, err = s.tasks.Move(ctx, id, models.StatusTesting); err != nil {
		return false, err
	}
	body := "The pull request was merged: " + prURL
	_, err = s.tasks.AddComment(ctx, id, systemAuthor, body)
	return true, err
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
		if err = s.pullRequestEnded(ctx, job, status); err != nil {
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
	status githubapp.PullRequestStatus,
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
	if status.State == githubapp.StateMerged {
		to, body = models.StatusTesting, "The pull request was merged: "
		err = s.tasks.SetMergeSHA(ctx, job.TaskID, status.MergeSHA)
	}
	// Move first: a failure after it loses a comment, where a failure before it
	// would repeat the comment on every poll until the move worked.
	if err == nil {
		_, err = s.tasks.Move(ctx, job.TaskID, to)
	}
	if err != nil {
		return err
	}
	_, err = s.tasks.AddComment(ctx, job.TaskID, systemAuthor, body+job.PRURL)
	return err
}

// syncLinked keeps the status of the open pull requests that link a task in their
// body, whoever opened them: those open now, then any it knew as open that have
// left the list (merged or closed since).
func (s *JobService) syncLinked(ctx context.Context) error {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return err
	}
	var failures []error
	seen := map[string]bool{}
	for i := range repos {
		pulls, listErr := s.github.OpenPullRequests(ctx, repos[i].Owner, repos[i].Name)
		if listErr != nil {
			failures = append(failures, fmt.Errorf("repo %s: %w", repos[i].Name, listErr))
			continue
		}
		for _, p := range pulls {
			ref, ok := linkedTask(p.Body)
			if !ok {
				continue
			}
			id, resolveErr := s.tasks.Resolve(ctx, ref)
			if resolveErr != nil {
				continue
			}
			seen[p.URL] = true
			if err = s.recordLinked(ctx, repos[i], id, p.URL, p.Number); err != nil {
				failures = append(failures, fmt.Errorf("pull request %s: %w", p.URL, err))
			}
		}
	}
	if err = s.refreshStale(ctx, seen); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

// refreshStale re-reads the pull requests known as open that the open lists no
// longer showed: they were merged or closed since.
func (s *JobService) refreshStale(ctx context.Context, seen map[string]bool) error {
	stale, err := s.linked.Open(ctx, pollBatch)
	if err != nil {
		return err
	}
	var failures []error
	for _, p := range stale {
		if seen[p.URL] {
			continue
		}
		task, getErr := s.tasks.Get(ctx, p.TaskID)
		if getErr != nil {
			continue
		}
		repo, getErr := s.repos.Get(ctx, task.RepoID)
		if getErr != nil {
			failures = append(failures, getErr)
			continue
		}
		number, ok := pullNumber(p.URL, repo)
		if !ok {
			continue
		}
		if err = s.recordLinked(ctx, *repo, p.TaskID, p.URL, number); err != nil {
			failures = append(failures, fmt.Errorf("pull request %s: %w", p.URL, err))
		}
	}
	return errors.Join(failures...)
}

func (s *JobService) recordLinked(
	ctx context.Context,
	repo models.Repo,
	taskID uuid.UUID,
	url string,
	number int,
) error {
	if _, err := s.tasks.Get(ctx, taskID); err != nil {
		// A link to a task that is gone is not a failure.
		return nil //nolint:nilerr // nothing to record
	}
	status, err := s.github.PullRequest(ctx, repo.Owner, repo.Name, number)
	if err != nil {
		return err
	}
	return s.linked.Upsert(ctx, models.LinkedPullRequest{
		URL: url, TaskID: taskID, State: status.State, CI: status.CI,
		Review: status.Review, Mergeable: status.Mergeable, UnmergeableSince: nil,
		CheckedAt: time.Time{},
	})
}

// PullRequestsOf lists every pull request of a task: those its jobs opened, then
// the others that link it in their body.
func (s *JobService) PullRequestsOf(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.LinkedPullRequest, error) {
	jobs, err := s.jobs.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	linked, err := s.linked.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	// A job's pull request is also read as a linked one: that read has its
	// mergeable state.
	mergeable := map[string]string{}
	for _, p := range linked {
		mergeable[p.URL] = p.Mergeable
	}
	out := []models.LinkedPullRequest{}
	seen := map[string]bool{}
	for i := range jobs {
		if jobs[i].PRURL == "" || seen[jobs[i].PRURL] {
			continue
		}
		seen[jobs[i].PRURL] = true
		out = append(out, models.LinkedPullRequest{
			URL: jobs[i].PRURL, TaskID: taskID, State: jobs[i].PRState,
			CI: jobs[i].PRCI, Review: jobs[i].PRReview,
			Mergeable:        mergeable[jobs[i].PRURL],
			UnmergeableSince: nil, CheckedAt: time.Time{},
		})
	}
	for _, p := range linked {
		if !seen[p.URL] {
			out = append(out, p)
		}
	}
	return out, nil
}
