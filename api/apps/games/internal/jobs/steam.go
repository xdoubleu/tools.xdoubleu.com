package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/games/internal/services"
	"tools.xdoubleu.com/internal/auth"
	internalmodels "tools.xdoubleu.com/internal/models"
)

type SteamJob struct {
	authService  auth.Service
	steamService *services.SteamService
}

func NewSteamJob(
	authService auth.Service,
	steamService *services.SteamService,
) SteamJob {
	return SteamJob{
		authService:  authService,
		steamService: steamService,
	}
}

func (j SteamJob) ID() string {
	return "steam"
}

func (j SteamJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j SteamJob) Run(ctx context.Context, logger *slog.Logger) error {
	users, err := j.authService.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, user := range users {
		if userErr := j.runForUser(ctx, logger, user); userErr != nil {
			logger.ErrorContext(ctx, "steam job failed for user",
				slog.String("userID", user.ID),
				slog.Any("error", userErr),
			)
			errs = append(errs, userErr)
		}
	}

	return errors.Join(errs...)
}

func (j SteamJob) runForUser(
	ctx context.Context,
	logger *slog.Logger,
	user internalmodels.User,
) error {
	logger.DebugContext(ctx, "syncing steam data", slog.String("userID", user.ID))
	return j.steamService.SyncUser(ctx, user.ID)
}
