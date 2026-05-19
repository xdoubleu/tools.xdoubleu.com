package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/helper"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/internal/auth"
	internalmodels "tools.xdoubleu.com/internal/models"
)

type SteamJob struct {
	authService     auth.Service
	steamService    *services.SteamService
	progressService *services.ProgressService
}

func NewSteamJob(
	authService auth.Service,
	steamService *services.SteamService,
	progressService *services.ProgressService,
) SteamJob {
	return SteamJob{
		authService:     authService,
		steamService:    steamService,
		progressService: progressService,
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
	users, err := j.authService.GetAllUsers()
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
	logger.DebugContext(ctx, "fetching owned games")
	ownedGames, err := j.steamService.ImportOwnedGames(ctx, user.ID)
	if err != nil {
		return err
	}
	if ownedGames == nil {
		logger.DebugContext(ctx, "steam not configured for user", "userID", user.ID)
		return nil
	}
	logger.DebugContext(ctx, fmt.Sprintf("fetched %d games", len(ownedGames)))

	gamesIDNameMap := map[int]string{}
	for _, game := range ownedGames {
		gamesIDNameMap[game.ID] = game.Name
	}

	achievementsForGame, err := j.steamService.GetAchievementsForGames(
		ctx, ownedGames, user.ID,
	)
	if err != nil {
		return err
	}

	totalAchievementsPerGame := map[int]int{}
	for _, game := range ownedGames {
		totalAchievementsPerGame[game.ID] = len(achievementsForGame[game.ID])
	}

	grapher := helper.NewAchievementsGrapher(totalAchievementsPerGame)

	totalAchievedAchievements := 0
	totalStartedGames := 0
	for gameID, achievements := range achievementsForGame {
		achievedForGame := 0
		for _, achievement := range achievements {
			if !achievement.Achieved {
				continue
			}
			achievedForGame++
			grapher.AddPoint(*achievement.UnlockTime, gameID)
		}
		if achievedForGame > 0 {
			logger.DebugContext(ctx, fmt.Sprintf(
				"achieved %d achievements in '%s'",
				achievedForGame, gamesIDNameMap[gameID],
			))
			totalStartedGames++
			totalAchievedAchievements += achievedForGame
		}
	}

	logger.DebugContext(ctx, fmt.Sprintf(
		"achieved %d achievements in %d games",
		totalAchievedAchievements, totalStartedGames,
	))

	progressLabels, progressValues := grapher.ToSlices()

	logger.DebugContext(ctx, "saving progress")
	return j.progressService.Save(
		ctx, models.SteamTypeID, user.ID, progressLabels, progressValues,
	)
}
