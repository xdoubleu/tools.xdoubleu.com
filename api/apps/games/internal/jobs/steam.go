package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/auth"
	internalmodels "tools.xdoubleu.com/internal/models"
)

// steamSyncer is the slice of services.SteamService SteamJob needs.
type steamSyncer interface {
	SyncUser(ctx context.Context, userID string) error
}

type SteamJob struct {
	authService  auth.Service
	steamService steamSyncer
}

func NewSteamJob(
	authService auth.Service,
	steamService steamSyncer,
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

	// A per-user sync failure (private profile, transient Steam API error, stale
	// steam id, ...) only skips that user -- it must not fail the whole job, since
	// that would stop global.job_runs' last-success timestamp from ever advancing
	// and make the job look due again on every restart. Each failure is still
	// logged at Error, which reaches Sentry on its own.
	for _, user := range users {
		if userErr := j.runForUser(ctx, logger, user); userErr != nil {
			logger.ErrorContext(ctx, "steam job failed for user",
				slog.String("userID", user.ID),
				slog.Any("error", userErr),
			)
		}
	}

	return nil
}

func (j SteamJob) runForUser(
	ctx context.Context,
	logger *slog.Logger,
	user internalmodels.User,
) error {
	logger.DebugContext(ctx, "syncing steam data", slog.String("userID", user.ID))
	return j.steamService.SyncUser(ctx, user.ID)
}
