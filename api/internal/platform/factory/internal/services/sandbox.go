package services

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// SandboxLabel marks a task about a problem with the job sandbox.
const SandboxLabel = "factory-sandbox"

// IsSandboxTask reports whether t carries the sandbox label.
func IsSandboxTask(t *models.Task) bool {
	return slices.ContainsFunc(t.Labels, func(l string) bool {
		return strings.EqualFold(l, SandboxLabel)
	})
}

// ReportSandbox files a sandbox problem: it comments on the repo's open sandbox
// task with the same title (ignoring case), else creates one in Backlog. created
// is false for a repeat.
func (s *TaskService) ReportSandbox(
	ctx context.Context,
	actor models.Actor,
	repoID uuid.UUID,
	title, body string,
) (*models.Task, bool, error) {
	in, err := normalise(TaskInput{
		Title: title, Body: body, Priority: models.PriorityP2,
		Labels: []string{SandboxLabel},
	})
	if err != nil {
		return nil, false, err
	}
	all, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, false, err
	}
	for i := range all {
		t := &all[i]
		if t.RepoID != repoID || t.Status == models.StatusDone || !IsSandboxTask(t) ||
			!strings.EqualFold(t.Title, in.Title) {
			continue
		}
		comment, cerr := normaliseComment(cmp.Or(in.Body, "Seen again."))
		if cerr != nil {
			return nil, false, cerr
		}
		_, err = s.repo.AddComment(ctx, t.ID, actor.String(), comment)
		return t, false, err
	}
	task, err := s.Create(ctx, actor, repoID, in)
	return task, err == nil, err
}

const maxBlockedTitle = 100

// ReportBlocked files what stopped a job, found by the runner, as a sandbox task
// in the job's repo (grouped like ReportSandbox). Only the runner that ran the job
// may report for it.
func (s *JobService) ReportBlocked(
	ctx context.Context,
	runnerID, jobID uuid.UUID,
	blocked string,
) error {
	blocked = strings.TrimSpace(blocked)
	if blocked == "" {
		return nil
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.RunnerID == nil || *job.RunnerID != runnerID {
		return conflict("the job belongs to another runner")
	}
	first, _, _ := strings.Cut(blocked, "\n")
	title := "Sandbox: " + strings.TrimSpace(first)
	if r := []rune(title); len(r) > maxBlockedTitle {
		title = string(r[:maxBlockedTitle])
	}
	body := "Reported by the runner for T-" + strconv.FormatInt(
		job.TaskNumber,
		10,
	) + " (job " +
		job.ID.String()[:shortJobIDLen] + "):\n\n" + blocked
	actor := models.Actor{
		Kind: models.ActorAgent, ID: "runner:" + runnerID.String(),
		Browser: false, Admin: false,
	}
	_, _, err = (&TaskService{repo: s.tasks}).ReportSandbox(
		ctx, actor, job.RepoID, title, body)
	return err
}
