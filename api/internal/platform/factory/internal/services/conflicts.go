package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	// conflictAuthor creates the jobs that merge the base into a conflicting
	// pull request.
	conflictAuthor = "system:conflict"
	// mergeConflicting is PullRequestStatus.Mergeable for a conflict with the base.
	mergeConflicting = "conflicting"
)

// conflictSignature identifies a conflict by the pull request's head: the same
// head again means the job left no new commit.
func conflictSignature(head string) string {
	return "conflict:" + head
}

// conflictPrompt is what a job that merges the base into a conflicting pull
// request is told; it ends with the conflict's signature.
func conflictPrompt(url, base, head string) string {
	return "The pull request " + url + " conflicts with " + base + ". Merge origin/" +
		base + " into the current branch (a merge, not a rebase: the runner pushes " +
		"a fast-forward), resolve the conflicts, take the base's version of " +
		"generated stubs (api/gen, web/lib/gen) and regenerate them with the " +
		"/local generators (`make proto/generate/local`, `npm run generate:local`), " +
		"run the gates, commit, and write the outcome again. Your work continues " +
		"until the pull request merges.\n\n(" + conflictSignature(head) + ")"
}

// resolveConflict queues a job on the branch of an armed pull request that
// conflicts with the base once no job is left to watch it. A second conflict at
// the same head, which the last such job left unresolved, moves the task to
// Needs you instead.
func (s *JobService) resolveConflict(ctx context.Context, r linkedReview) error {
	if r.status.Mergeable != mergeConflicting || r.status.HeadSHA == "" {
		return nil
	}
	task, err := s.tasks.Get(ctx, r.task.ID)
	if err != nil || task.Status != models.StatusInReview {
		return nil //nolint:nilerr // a task that is gone is not a failure
	}
	jobs, err := s.jobs.ByTask(ctx, task.ID)
	if err != nil || len(jobs) == 0 || hasActive(jobs) || jobs[0].Branch != r.branch {
		return err
	}
	sig := conflictSignature(r.status.HeadSHA)
	if jobs[0].CreatedBy == conflictAuthor && strings.Contains(jobs[0].Prompt, sig) {
		return s.finishTask(ctx, task.ID, systemAuthor,
			"The pull request "+r.url+" still conflicts with "+r.repo.DefaultBranch+
				" and the job that tried to merge it left no new commit ("+sig+
				").", models.StatusNeedsYou)
	}
	prompt := conflictPrompt(r.url, r.repo.DefaultBranch, r.status.HeadSHA)
	var parent *uuid.UUID
	if i := indexWithSession(jobs); i >= 0 {
		parent = &jobs[i].ID
	} else {
		prompt = promptFor(task) + "\n\n" + prompt
	}
	if _, err = s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, Kind: "", TaskID: task.ID, TaskNumber: 0, RepoID: jobs[0].RepoID,
		Harness: "", Model: "", Prompt: prompt, Status: "", Branch: r.branch,
		SessionID: "", Outcome: "", ParentJobID: parent, PRURL: "", RunnerID: nil,
		PRState: "", PRCI: "", PRReview: "", CreatedBy: conflictAuthor,
		CreatedAt: time.Time{}, Routine: "", Limited: false, LimitResetsAt: nil,
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	}); err != nil {
		return err
	}
	return s.finishTask(ctx, task.ID, systemAuthor,
		"The pull request "+r.url+" conflicts with "+r.repo.DefaultBranch+
			" and no job is working on it; a job merges the base on its branch.",
		models.StatusInProgress)
}

// indexWithSession is the index of the newest job that has a harness session to
// resume, or -1.
func indexWithSession(jobs []models.Job) int {
	for i := range jobs {
		if jobs[i].SessionID != "" {
			return i
		}
	}
	return -1
}
