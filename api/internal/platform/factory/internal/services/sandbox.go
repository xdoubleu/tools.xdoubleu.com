package services

import (
	"cmp"
	"context"
	"regexp"
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

// ReportSandbox files a sandbox problem: it comments on the open task the title
// names (T-n), else the repo's open sandbox task with the same or a close title,
// else creates one (straight into Ready, as a bug). created is false for a repeat.
func (s *TaskService) ReportSandbox(
	ctx context.Context,
	actor models.Actor,
	repoID uuid.UUID,
	title, body string,
) (*models.Task, bool, error) {
	in, err := normalise(TaskInput{
		Title: title, Body: body, Priority: models.PriorityP2,
		Labels: []string{SandboxLabel, "bug"},
	})
	if err != nil {
		return nil, false, err
	}
	all, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, false, err
	}
	if t := findCovering(all, repoID, in.Title); t != nil {
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

// waitingPattern matches wording for work that does not exist yet, which is a
// dependency on other tasks, not a sandbox gap.
var waitingPattern = regexp.MustCompile(
	`(?i)\b(doesn't|does not|don't|do not|not) exist yet\b`)

// isSandboxGap reports whether a Blocked line can be a sandbox gap: one naming
// tasks or work that does not exist yet is a dependency.
func isSandboxGap(blocked string) bool {
	return !taskRefPattern.MatchString(blocked) && !waitingPattern.MatchString(blocked)
}

// waitingRefs normalises refs to distinct "T-n" references, keeping their order.
func waitingRefs(refs []string) []string {
	var out []string
	for _, r := range refs {
		for _, m := range taskRefPattern.FindAllString(r, -1) {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}

// ReportBlocked handles what stopped a job, found by the runner. A task waiting on
// other tasks (waitingOn, or a Blocked line naming them) goes to Backlog with a
// comment and nothing is filed; a sandbox gap becomes a sandbox task in the job's
// repo (grouped like ReportSandbox). Only the runner that ran the job may report
// for it.
func (s *JobService) ReportBlocked(
	ctx context.Context,
	runnerID, jobID uuid.UUID,
	blocked string,
	waitingOn []string,
) error {
	blocked = strings.TrimSpace(blocked)
	waiting := waitingRefs(waitingOn)
	if len(waiting) == 0 && !isSandboxGap(blocked) {
		waiting = waitingRefs([]string{blocked})
	}
	if blocked == "" && len(waiting) == 0 {
		return nil
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.RunnerID == nil || *job.RunnerID != runnerID {
		return conflict("the job belongs to another runner")
	}
	if len(waiting) > 0 {
		return s.backlogWaiting(ctx, job, waiting)
	}
	if !isSandboxGap(blocked) {
		return nil
	}
	first, _, _ := strings.Cut(blocked, "\n")
	title := "Sandbox: " + strings.TrimSpace(first)
	who := "T-" + strconv.FormatInt(job.TaskNumber, 10)
	if job.IsRoutine() {
		// One task per routine, not one per run: the text of a run varies.
		title = "Sandbox: routine " + job.Routine + " is blocked"
		who = "routine " + job.Routine
	}
	if r := []rune(title); len(r) > maxBlockedTitle {
		title = string(r[:maxBlockedTitle])
	}
	body := "Reported by the runner for " + who + " (job " +
		job.ID.String()[:shortJobIDLen] + "):\n\n" + blocked
	actor := models.Actor{
		Kind: models.ActorAgent, ID: "runner:" + runnerID.String(),
		Browser: false, Admin: false, Via: "",
	}
	tasks := &TaskService{repo: s.tasks}
	if noImageFix(blocked) {
		// No new image can unblock it: a bug to fix, not a task to park on.
		_, _, err = tasks.reportBug(ctx, actor, job.RepoID,
			strings.TrimPrefix(title, "Sandbox: "), body)
		return err
	}
	sandbox, _, err := tasks.ReportSandbox(ctx, actor, job.RepoID, title, body)
	if err == nil && !job.IsRoutine() && sandbox.ID != job.TaskID {
		err = s.park(ctx, job, sandbox)
	}
	return err
}

// noImagePattern matches an agent saying that no change to the job image fixes
// its gap.
var noImagePattern = regexp.MustCompile(
	`(?i)\bno (containerfile|image|dockerfile) (fix|change)\b|` +
		`\b(image|containerfile) (change|fix) (can't|cannot|won't|would not|will not)\b`)

// noImageFix reports whether a Blocked line says no image change can fix its gap.
func noImageFix(blocked string) bool {
	return noImagePattern.MatchString(blocked)
}

// reportBug files a plain bug task (straight into Ready) unless an open task in
// the repo already has the title. created is false for a repeat.
func (s *TaskService) reportBug(
	ctx context.Context,
	actor models.Actor,
	repoID uuid.UUID,
	title, body string,
) (*models.Task, bool, error) {
	in, err := normalise(TaskInput{
		Title: title, Body: body, Priority: models.PriorityP2,
		Labels: []string{"bug"},
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
		if t.RepoID == repoID && t.Status != models.StatusDone &&
			strings.EqualFold(t.Title, in.Title) {
			return t, false, nil
		}
	}
	task, err := s.Create(ctx, actor, repoID, in)
	return task, err == nil, err
}

// backlogWaiting moves a task whose job found it depends on other tasks to
// Backlog, naming them. A task with a pull request stays where it is.
func (s *JobService) backlogWaiting(
	ctx context.Context,
	job *models.Job,
	waiting []string,
) error {
	if job.PRURL != "" {
		return nil
	}
	return s.finishTask(ctx, job.TaskID, systemAuthor,
		"Waiting on "+strings.Join(waiting, ", ")+
			"; moved to Backlog until they are done.",
		models.StatusBacklog)
}
