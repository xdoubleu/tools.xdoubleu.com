package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/goaltracker/internal/helper"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/internal/services"
	"tools.xdoubleu.com/internal/auth"
)

type SteamJob struct {
	authService  auth.Service
	steamService *services.SteamService
	goalService  *services.GoalService
}

func NewSteamJob(
	authService auth.Service,
	steamService *services.SteamService,
	goalService *services.GoalService,
) SteamJob {
	return SteamJob{
		authService:  authService,
		steamService: steamService,
		goalService:  goalService,
	}
}

func (j SteamJob) ID() string {
	return strconv.Itoa(int(models.SteamSource.ID))
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
		logger.Debug("fetching owned games")
		var ownedGames []models.Game
		ownedGames, err = j.steamService.ImportOwnedGames(ctx, user.ID)
		if err != nil {
			return err
		}
		logger.Debug(
			fmt.Sprintf("fetched %d games", len(ownedGames)),
		)

		gamesIDNameMap := map[int]string{}
		for _, game := range ownedGames {
			gamesIDNameMap[game.ID] = game.Name
		}

		achievementsPerGame := map[int][]models.Achievement{}
		totalAchievementsPerGame := map[int]int{}

		var achievementsForGame map[int][]models.Achievement
		achievementsForGame, err = j.steamService.GetAchievementsForGames(
			ctx,
			ownedGames,
			user.ID,
		)
		if err != nil {
			return err
		}

		for _, game := range ownedGames {
			achievementsPerGame[game.ID] = achievementsForGame[game.ID]
			totalAchievementsPerGame[game.ID] = len(achievementsForGame[game.ID])
		}

		grapher := helper.NewAchievementsGrapher(totalAchievementsPerGame)

		totalAchievedAchievements := 0
		totalStartedGames := 0
		for gameID, achievements := range achievementsPerGame {
			achievedForGame := 0
			for _, achievement := range achievements {
				if !achievement.Achieved {
					continue
				}

				achievedForGame++

				grapher.AddPoint(*achievement.UnlockTime, gameID)
			}

			if achievedForGame > 0 {
				logger.Debug(
					fmt.Sprintf(
						"achieved %d achievements in '%s'",
						achievedForGame,
						gamesIDNameMap[gameID],
					),
				)
				totalStartedGames++
				totalAchievedAchievements += achievedForGame
			}
		}

		logger.Debug(
			fmt.Sprintf(
				"achieved %d achievements in %d games",
				totalAchievedAchievements,
				totalStartedGames,
			),
		)

		progressLabels, progressValues := grapher.ToSlices()

		logger.Debug("saving progress")
		err = j.goalService.SaveProgress(
			ctx,
			models.SteamCompletionRate.ID,
			user.ID,
			progressLabels,
			progressValues,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
