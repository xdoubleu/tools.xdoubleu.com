package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/goaltracker/internal/services"
	"tools.xdoubleu.com/internal/auth"
)

type TodoistJob struct {
	authService auth.Service
	goalService *services.GoalService
}

func NewTodoistJob(
	authService auth.Service,
	goalService *services.GoalService,
) TodoistJob {
	return TodoistJob{
		authService: authService,
		goalService: goalService,
	}
}

func (j TodoistJob) ID() string {
	return "todoist"
}

func (j TodoistJob) RunEvery() time.Duration {
	//nolint:mnd //no magic number
	return 24 * time.Hour
}

func (j TodoistJob) Run(ctx context.Context, logger *slog.Logger) error {
	users, err := j.authService.GetAllUsers()
	if err != nil {
		return err
	}

	for _, user := range users {
		logger.Debug("importing states")
		err = j.goalService.ImportStatesFromTodoist(ctx, user.ID)
		if err != nil {
			return err
		}

		logger.Debug("importing goals")
		err = j.goalService.ImportGoalsFromTodoist(ctx, user.ID)
		if err != nil {
			return err
		}
	}

	return nil
}
