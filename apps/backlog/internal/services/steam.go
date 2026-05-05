package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
)

type SteamService struct {
	logger        *slog.Logger
	clientFactory func(apiKey string) steam.Client
	steam         *repositories.SteamRepository
	integrations  *IntegrationsService
}

func (service *SteamService) ImportOwnedGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	creds, err := service.integrations.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if creds.SteamAPIKey == "" || creds.SteamUserID == "" {
		return nil, nil
	}

	client := service.clientFactory(creds.SteamAPIKey)

	ownedGamesResponse, err := client.GetOwnedGames(ctx, creds.SteamUserID)
	if err != nil {
		return nil, err
	}

	gamesMap := map[int]*models.Game{}
	for _, game := range ownedGamesResponse.Response.Games {
		gamesMap[game.AppID] = &models.Game{
			ID:             game.AppID,
			Name:           game.Name,
			Playtime:       game.PlaytimeForever,
			CompletionRate: "0.00",
			Contribution:   "0.0000",
			IsDelisted:     false,
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
		service.logger.DebugContext(
			ctx,
			fmt.Sprintf("game '%s' (%d) is delisted", game.Name, game.ID),
		)
		game.IsDelisted = true
		gamesMap[game.ID] = &game
	}

	err = service.steam.UpsertGames(ctx, gamesMap, userID)
	if err != nil {
		return nil, err
	}

	achievementsPerGame, err := service.importAchievementsForGames(
		ctx,
		client,
		creds.SteamUserID,
		gamesMap,
		userID,
	)
	if err != nil {
		return nil, err
	}

	for ID := range gamesMap {
		service.logger.DebugContext(
			ctx,
			fmt.Sprintf(
				"calculating completion rate for '%s' (%d) with %d achievements",
				gamesMap[ID].Name,
				gamesMap[ID].ID,
				len(achievementsPerGame[ID]),
			),
		)
		gamesMap[ID].SetCalculatedInfo(achievementsPerGame[ID], len(gamesMap))
	}

	err = service.steam.UpsertGames(ctx, gamesMap, userID)
	if err != nil {
		return nil, err
	}

	return service.steam.GetAllGames(ctx, userID)
}

func (service *SteamService) importAchievementsForGames(
	ctx context.Context,
	client steam.Client,
	steamUserID string,
	gamesMap map[int]*models.Game,
	userID string,
) (map[int][]models.Achievement, error) {
	gameIDs := []int{}
	for ID := range gamesMap {
		gameIDs = append(gameIDs, ID)
	}

	const gamesPerWorker = 10
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(gamesPerWorker)

	mu := sync.Mutex{}
	achievementsPerGame := map[int][]steam.Achievement{}
	for _, ID := range gameIDs {
		eg.Go(func() error {
			achievementsForGame, errIn := client.GetPlayerAchievements(
				egCtx,
				steamUserID,
				ID,
			)
			if errIn != nil {
				service.logger.WarnContext(
					egCtx,
					fmt.Sprintf(
						"failed to fetch achievements for %d; error: %s",
						ID,
						errIn,
					),
				)
				return nil
			}

			mu.Lock()
			achievementsPerGame[ID] = achievementsForGame.PlayerStats.Achievements
			mu.Unlock()

			service.logger.DebugContext(
				egCtx,
				fmt.Sprintf(
					"fetched %d achievements for '%s' (%d)",
					len(achievementsForGame.PlayerStats.Achievements),
					gamesMap[ID].Name,
					ID,
				),
			)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	for gameID, achievements := range achievementsPerGame {
		if err := service.upsertAchievementsForGame(
			ctx, client, achievements, userID, gameID,
		); err != nil {
			return nil, err
		}
	}

	return service.steam.GetAchievementsForGames(ctx, gameIDs, userID)
}

func (service *SteamService) upsertAchievementsForGame(
	ctx context.Context,
	client steam.Client,
	achievements []steam.Achievement,
	userID string,
	gameID int,
) error {
	globalPercents, err := fetchGlobalPercents(ctx, client, gameID)
	if err != nil {
		service.logger.WarnContext(
			ctx,
			fmt.Sprintf(
				"failed to fetch global percents for %d; error: %s",
				gameID,
				err,
			),
		)
		globalPercents = map[string]float64{}
	}

	schemaResp, err := client.GetSchemaForGame(ctx, gameID)
	if err != nil {
		return err
	}

	schemaMap := make(
		map[string]steam.AchievementSchema,
		len(schemaResp.Game.AvailableGameStats.Achievements),
	)
	for _, s := range schemaResp.Game.AvailableGameStats.Achievements {
		schemaMap[s.Name] = s
	}

	if len(achievements) != 0 {
		return service.steam.UpsertAchievements(
			ctx, achievements, globalPercents, schemaMap, userID, gameID,
		)
	}

	return service.steam.UpsertAchievementSchemas(
		ctx,
		schemaResp.Game.AvailableGameStats.Achievements,
		globalPercents,
		userID,
		gameID,
	)
}

func fetchGlobalPercents(
	ctx context.Context,
	client steam.Client,
	gameID int,
) (map[string]float64, error) {
	resp, err := client.GetGlobalAchievementPercentagesForApp(ctx, gameID)
	if err != nil {
		return nil, err
	}

	percents := make(map[string]float64, len(resp.AchievementPercentages.Achievements))
	for _, a := range resp.AchievementPercentages.Achievements {
		p, parseErr := a.Percent.Float64()
		if parseErr == nil {
			percents[a.Name] = p
		}
	}

	return percents, nil
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

func (service *SteamService) GetBacklog(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetBacklog(ctx, userID)
}

func (service *SteamService) GetInProgress(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetInProgress(ctx, userID)
}

func (service *SteamService) GetCompleted(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetCompleted(ctx, userID)
}

func (service *SteamService) GetGameByID(
	ctx context.Context,
	gameID int,
	userID string,
) (*models.Game, error) {
	return service.steam.GetGameByID(ctx, gameID, userID)
}

func (service *SteamService) GetAchievementsForGame(
	ctx context.Context,
	gameID int,
	userID string,
) ([]models.Achievement, error) {
	achievementsMap, err := service.steam.GetAchievementsForGames(
		ctx,
		[]int{gameID},
		userID,
	)
	if err != nil {
		return nil, err
	}
	return achievementsMap[gameID], nil
}
