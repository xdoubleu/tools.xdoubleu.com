package services

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// waitResumeAuthor creates the jobs that resume tasks whose wait is over.
const waitResumeAuthor = "system:waiting"

const waitResumePrompt = "The tasks you were waiting on are merged. Update your " +
	"branch from main, finish the work, and write the outcome."

// ResumeWaiting resumes every Backlog task whose waited-on tasks have all merged:
// a job continues its newest session on its branch, so the run that was approved
// goes on. A task with no session goes to Needs you. A task split into slices has
// nothing left to do once they merge: it goes to Done.
func (s *JobService) ResumeWaiting(ctx context.Context) (int, error) {
	ids, err := s.tasks.WaitOver(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, id := range ids {
		if err = s.resumeWaiting(ctx, id); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

func (s *JobService) resumeWaiting(ctx context.Context, taskID uuid.UUID) error {
	jobs, err := s.jobs.ByTask(ctx, taskID)
	if err != nil {
		return err
	}
	if hasActive(jobs) {
		return s.tasks.SetWaitingOn(ctx, taskID, nil)
	}
	if done, splitErr := s.resolveSplit(ctx, taskID); done || splitErr != nil {
		return splitErr
	}
	// Jobs are newest first: resume the newest session there is.
	i := slices.IndexFunc(jobs, func(j models.Job) bool { return j.SessionID != "" })
	if i < 0 {
		return s.finishTask(ctx, taskID, systemAuthor,
			"The tasks this one waited on are merged, but there is no session to "+
				"resume. Move it to Ready to start it again.",
			models.StatusNeedsYou)
	}
	last := jobs[i]
	if _, err = s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, Kind: "", TaskID: taskID, TaskNumber: 0, RepoID: last.RepoID,
		Harness: "", Model: "", Prompt: waitResumePrompt,
		Status: "", Branch: last.Branch, SessionID: "", Outcome: "",
		ParentJobID: &last.ID, PRURL: "", RunnerID: nil, PRState: "", PRCI: "",
		PRReview: "", CreatedBy: waitResumeAuthor, CreatedAt: time.Time{},
		Routine: "", Limited: false, LimitResetsAt: nil,
		FinishedHarness: "", FinishedModel: "",
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	}); err != nil {
		return err
	}
	// The move clears waiting_on, so a failure above leaves the task to retry.
	return s.finishTask(ctx, taskID, systemAuthor,
		"The tasks this one waited on are merged; it resumes on its branch.",
		models.StatusInProgress)
}

// epicLabel prefixes the label a slice carries.
const epicLabel = "epic:"

// resolveSplit moves a task to Done when everything it waited on is a slice
// filed after it (an epic: task), reporting whether it did.
func (s *JobService) resolveSplit(ctx context.Context, taskID uuid.UUID) (bool, error) {
	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return false, err
	}
	parts, err := s.tasks.WaitingOn(ctx, taskID)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	refs := make([]string, 0, len(parts))
	for i := range parts {
		sl := &parts[i]
		if !sl.CreatedAt.After(task.CreatedAt) || !isSlice(sl) {
			return false, nil
		}
		refs = append(refs, "T-"+strconv.FormatInt(sl.Number, 10))
	}
	return true, s.finishTask(ctx, taskID, systemAuthor,
		"Split into "+strings.Join(refs, ", ")+", all merged; nothing is left here.",
		models.StatusDone)
}

func isSlice(t *models.Task) bool {
	return slices.ContainsFunc(t.Labels, func(l string) bool {
		return strings.HasPrefix(strings.ToLower(l), epicLabel)
	})
}
