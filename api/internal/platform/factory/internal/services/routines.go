package services

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/slack"
)

const (
	// scheduleAuthor is who a job the schedule queued is attributed to.
	scheduleAuthor = "system:schedule"
	// maxRoutineInterval is a month, in minutes.
	maxRoutineInterval = 31 * 24 * 60
)

var routineNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

// RoutineInput is the editable part of a routine.
type RoutineInput struct {
	Name            string
	RepoID          uuid.UUID
	Enabled         bool
	IntervalMinutes int
}

// routineBranch is the branch a routine's job works on in the runner's clone. It
// is never pushed.
func routineBranch(name string) string {
	return "agent/routine-" + name
}

// routinePrompt is what the server asks for; the runner builds its own from the
// routine's name, so this only says what the job is on the board.
func routinePrompt(repo *models.Repo, name string) string {
	return repo.AgentInstructions + "\n\n# Routine: " + name
}

// SaveRoutine creates a routine or changes its repo, flag and interval. Enabling a
// routine is the approval for it to run unattended, so only a human admin in a
// browser may, as for Ready.
func (s *JobService) SaveRoutine(
	ctx context.Context,
	actor models.Actor,
	in RoutineInput,
) (*models.Routine, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can change a routine")
	}
	if !routineNamePattern.MatchString(in.Name) {
		return nil, badRequest("a routine name is lower-case letters, digits and dashes")
	}
	if in.IntervalMinutes < 0 || in.IntervalMinutes > maxRoutineInterval {
		return nil, badRequest("the interval must be between 0 and a month")
	}
	if _, err := s.repos.Get(ctx, in.RepoID); err != nil {
		return nil, err
	}
	return s.routines.Save(ctx, models.Routine{
		Name: in.Name, RepoID: in.RepoID, Enabled: in.Enabled,
		IntervalMinutes: in.IntervalMinutes, CreatedAt: time.Time{}, LastJob: nil,
	})
}

// ListRoutines returns every routine with its newest job.
func (s *JobService) ListRoutines(ctx context.Context) ([]models.Routine, error) {
	list, err := s.routines.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		last, lastErr := s.jobs.LastForRoutine(ctx, list[i].Name)
		if errors.Is(lastErr, database.ErrResourceNotFound) {
			continue
		}
		if lastErr != nil {
			return nil, lastErr
		}
		list[i].LastJob = last
	}
	return list, nil
}

// RunRoutine queues a job for the routine now, by hand, even when it is disabled
// or unscheduled. Only a human admin in a browser may, as for Run.
func (s *JobService) RunRoutine(
	ctx context.Context,
	actor models.Actor,
	name string,
) (*models.Job, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can run a routine")
	}
	routine, err := s.routines.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.queueRoutine(ctx, routine, actor.String())
}

// queueRoutine adds the routine's job; one already queued or running is a conflict.
func (s *JobService) queueRoutine(
	ctx context.Context,
	routine *models.Routine,
	createdBy string,
) (*models.Job, error) {
	repo, err := s.repos.Get(ctx, routine.RepoID)
	if err != nil {
		return nil, err
	}
	harness, model, err := resolveHarness(repo, HarnessPick{Harness: "", Model: ""})
	if err != nil {
		return nil, err
	}
	job, err := s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, TaskID: uuid.Nil, TaskNumber: 0, Routine: routine.Name,
		RepoID: repo.ID, Harness: harness, Model: model,
		Prompt: routinePrompt(repo, routine.Name), Status: "",
		Branch: routineBranch(routine.Name), SessionID: "", Outcome: "",
		ParentJobID: nil, PRURL: "", RunnerID: nil, PRState: "", PRCI: "",
		PRReview: "", CreatedBy: createdBy, CreatedAt: time.Time{},
		Limited: false, LimitResetsAt: nil,
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	})
	if errors.Is(err, database.ErrResourceConflict) {
		return nil, conflict("this routine already has a job queued or running")
	}
	return job, err
}

// EnqueueDueRoutines queues a job for each enabled routine whose interval has run
// out and that has none queued or running, so runs missed while no runner was
// online coalesce into one. Nothing is queued while the board is paused.
func (s *JobService) EnqueueDueRoutines(ctx context.Context) error {
	paused, err := s.settings.PickupPaused(ctx)
	if err != nil || paused {
		return err
	}
	due, err := s.routines.Due(ctx)
	if err != nil {
		return err
	}
	var firstErr error
	for i := range due {
		// A job that appeared since the query is the one the routine was due for.
		_, queueErr := s.queueRoutine(ctx, &due[i], scheduleAuthor)
		if queueErr != nil && !isConflict(queueErr) && firstErr == nil {
			firstErr = queueErr
		}
	}
	return firstErr
}

func isConflict(err error) bool {
	var httpErr *app.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == http.StatusConflict
}

// StopRoutine cancels the routine's queued or running job. A running job's runner
// learns of it on its next check-in. Only a human admin in a browser may, as for
// Stop.
func (s *JobService) StopRoutine(
	ctx context.Context,
	actor models.Actor,
	name string,
) (*models.Job, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can stop a job")
	}
	job, err := s.jobs.CancelRoutine(ctx, name)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, conflict("this routine has no queued or running job")
	}
	if err != nil {
		return nil, err
	}
	s.revokeAccount(ctx, job.ID)
	s.say.say(ctx, ":octagonal_sign: Routine "+name+" was stopped by "+actor.String()+".")
	return job, nil
}

// endRoutine announces how a routine's job ended; it has no task to comment on.
func (s *JobService) endRoutine(
	ctx context.Context,
	job *models.Job,
	succeeded bool,
	outcome string,
) {
	switch {
	case job.Limited:
		s.say.say(ctx, ":hourglass_flowing_sand: Claude's usage limit stopped routine "+
			job.Routine+limitUntil(job.LimitResetsAt))
	case succeeded:
		s.say.say(ctx, ":white_check_mark: Routine "+job.Routine+" ran: "+
			slack.Escape(firstLine(outcome)))
	default:
		s.say.say(ctx, ":x: Routine "+job.Routine+" failed: "+
			slack.Escape(firstLine(outcome)))
	}
}
