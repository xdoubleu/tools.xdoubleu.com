package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/helper"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/internal/auth"
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
	//nolint:mnd //no magic number
	return 24 * time.Hour
}

//nolint:gocognit // this is fine for now
func (j SteamJob) Run(ctx context.Context, logger *slog.Logger) error {
	users, err := j.authService.GetAllUsers()
	if err != nil {
		return err
	}

	for _, user := range users {
		logger.DebugContext(ctx, "fetching owned games")
		var ownedGames []models.Game
		ownedGames, err = j.steamService.ImportOwnedGames(ctx, user.ID)
		if err != nil {
			return err
		}
		if ownedGames == nil {
			logger.DebugContext(ctx, "steam not configured for user", "userID", user.ID)
			continue
		}
		logger.DebugContext(ctx, fmt.Sprintf("fetched %d games", len(ownedGames)))

		gamesIDNameMap := map[int]string{}
		for _, game := range ownedGames {
			gamesIDNameMap[game.ID] = game.Name
		}

		achievementsForGame, fetchErr := j.steamService.GetAchievementsForGames(
			ctx, ownedGames, user.ID,
		)
		if fetchErr != nil {
			return fetchErr
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
		err = j.progressService.Save(
			ctx, models.SteamTypeID, user.ID, progressLabels, progressValues,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
