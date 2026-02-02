package services

import (
	"context"
	"log/slog"
	"sync"

	"github.com/xdoubleu/essentia/v2/pkg/threading"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
)

type SteamService struct {
	logger *slog.Logger
	client steam.Client
	userID string
	steam  *repositories.SteamRepository
}

func (service *SteamService) ImportOwnedGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	ownedGamesResponse, err := service.client.GetOwnedGames(ctx, service.userID)
	if err != nil {
		return nil, err
	}

	gamesMap := map[int]*models.Game{}
	for _, game := range ownedGamesResponse.Response.Games {
		//nolint:exhaustruct //others are defined later
		gamesMap[game.AppID] = &models.Game{
			ID:         game.AppID,
			Name:       game.Name,
			IsDelisted: false,
		}
	}

	games, err := service.steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, game := range games {
		_, ok := gamesMap[game.ID]

		if ok {
			continue
		}

		game.IsDelisted = true
		gamesMap[game.ID] = &game
	}

	achievementsPerGame, err := service.importAchievementsForGames(
		ctx,
		gamesMap,
		userID,
	)
	if err != nil {
		return nil, err
	}

	for ID := range gamesMap {
		gamesMap[ID].SetCalculatedInfo(achievementsPerGame[ID], len(gamesMap))
	}

	err = service.steam.UpsertGames(
		ctx,
		gamesMap,
		userID,
	)
	if err != nil {
		return nil, err
	}

	return service.steam.GetAllGames(ctx, userID)
}

func (service *SteamService) importAchievementsForGames(
	ctx context.Context,
	gamesMap map[int]*models.Game,
	userID string,
) (map[int][]models.Achievement, error) {
	gameIDs := []int{}
	for ID := range gamesMap {
		gameIDs = append(gameIDs, ID)
	}

	//nolint:mnd //no magic number
	amountWorkers := (len(gameIDs) / 10) + 1
	workerPool := threading.NewWorkerPool(service.logger, amountWorkers, len(gameIDs))

	mu := sync.Mutex{}
	achievementsPerGame := map[int][]steam.Achievement{}
	for _, ID := range gameIDs {
		workerPool.EnqueueWork(func(ctx context.Context, _ *slog.Logger) error {
			achievementsForGame, errIn := service.client.GetPlayerAchievements(
				ctx,
				service.userID,
				ID,
			)
			if errIn != nil {
				return errIn
			}

			mu.Lock()
			achievementsPerGame[ID] = achievementsForGame.PlayerStats.Achievements
			mu.Unlock()

			return nil
		})
	}

	workerPool.WaitUntilDone()

	for gameID, achievements := range achievementsPerGame {
		if len(achievements) != 0 {
			err := service.steam.UpsertAchievements(
				ctx,
				achievements,
				userID,
				gameID,
			)
			if err != nil {
				return nil, err
			}

			continue
		}

		var achievementSchemasForGame *steam.GetSchemaForGameResponse
		achievementSchemasForGame, err := service.client.GetSchemaForGame(ctx, gameID)
		if err != nil {
			return nil, err
		}

		err = service.steam.UpsertAchievementSchemas(
			ctx,
			achievementSchemasForGame.Game.AvailableGameStats.Achievements,
			userID,
			gameID,
		)
		if err != nil {
			return nil, err
		}
	}

	return service.steam.GetAchievementsForGames(ctx, gameIDs, userID)
}

func (service *SteamService) GetAchievementsForGames(
	ctx context.Context,
	games []models.Game,
	userID string,
) (map[int][]models.Achievement, error) {
	gameIDs := []int{}
	for _, game := range games {
		gameIDs = append(gameIDs, game.ID)
	}

	return service.steam.GetAchievementsForGames(ctx, gameIDs, userID)
}

func (service *SteamService) GetAllGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetAllGames(ctx, userID)
}
