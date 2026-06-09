package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/backlog/internal/helper"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
)

type SteamService struct {
	logger        *slog.Logger
	clientFactory func(apiKey string) steam.Client
	steam         *repositories.SteamRepository
	progress      *repositories.ProgressRepository
	integrations  *IntegrationsService
}

// SyncUser refreshes a user's Steam data. It fetches everything from the Steam
// API and computes the derived values (completion rate, contribution, progress
// graph) in memory first, then persists games, achievements and progress in a
// single transaction. On any error nothing is committed, so the previously
// stored — consistent — data is preserved. A per-game fetch failure only skips
// that game (its existing values are kept) instead of aborting the whole sync.
func (service *SteamService) SyncUser(ctx context.Context, userID string) error {
	creds, err := service.integrations.Get(ctx, userID)
	if err != nil {
		return err
	}
	if creds.SteamAPIKey == "" || creds.SteamUserID == "" {
		service.logger.DebugContext(
			ctx,
			"steam not configured for user",
			"userID",
			userID,
		)
		return nil
	}

	client := service.clientFactory(creds.SteamAPIKey)

	gamesMap, err := service.buildGamesMap(ctx, client, creds.SteamUserID, userID)
	if err != nil {
		return err
	}

	fetched := service.fetchAchievements(ctx, client, creds.SteamUserID, gamesMap)

	for id := range gamesMap {
		rows, ok := fetched[id]
		if !ok {
			// Fetch failed/skipped for this game: keep its existing values.
			continue
		}
		gamesMap[id].SetCalculatedInfo(rows, len(gamesMap))
	}

	labels, values := buildProgress(fetched)

	return service.steam.WithTx(ctx, func(tx pgx.Tx) error {
		if errIn := service.steam.UpsertGames(ctx, tx, gamesMap, userID); errIn != nil {
			return errIn
		}
		for gameID, rows := range fetched {
			if errIn := service.steam.ReplaceAchievements(
				ctx, tx, userID, gameID, rows,
			); errIn != nil {
				return errIn
			}
		}
		return service.progress.Upsert(
			ctx, tx, models.SteamTypeID, userID, labels, values,
		)
	})
}

// buildGamesMap merges the currently owned games with the games already stored
// for the user. Owned games seed their completion rate / contribution from the
// stored record so a later failed achievement fetch preserves those values;
// games no longer owned are carried over and marked delisted.
func (service *SteamService) buildGamesMap(
	ctx context.Context,
	client steam.Client,
	steamUserID string,
	userID string,
) (map[int]*models.Game, error) {
	ownedResp, err := client.GetOwnedGames(ctx, steamUserID)
	if err != nil {
		return nil, err
	}

	existing, err := service.steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, err
	}
	existingByID := make(map[int]models.Game, len(existing))
	for _, g := range existing {
		existingByID[g.ID] = g
	}

	gamesMap := map[int]*models.Game{}
	for _, g := range ownedResp.Response.Games {
		game := models.Game{
			ID:             g.AppID,
			Name:           g.Name,
			Playtime:       g.PlaytimeForever,
			CompletionRate: "0.00",
			Contribution:   "0.0000",
			IsDelisted:     false,
		}
		if prev, ok := existingByID[g.AppID]; ok {
			game.CompletionRate = prev.CompletionRate
			game.Contribution = prev.Contribution
		}
		gamesMap[g.AppID] = &game
	}

	for _, g := range existing {
		if _, ok := gamesMap[g.ID]; ok {
			continue
		}
		service.logger.DebugContext(
			ctx,
			fmt.Sprintf("game '%s' (%d) is delisted", g.Name, g.ID),
		)
		delisted := g
		delisted.IsDelisted = true
		gamesMap[g.ID] = &delisted
	}

	return gamesMap, nil
}

// fetchAchievements concurrently fetches and assembles the achievement rows for
// every game. A game whose fetch fails is logged and omitted from the result so
// its stored data is left untouched.
func (service *SteamService) fetchAchievements(
	ctx context.Context,
	client steam.Client,
	steamUserID string,
	gamesMap map[int]*models.Game,
) map[int][]models.Achievement {
	const gamesPerWorker = 10
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(gamesPerWorker)

	mu := sync.Mutex{}
	result := map[int][]models.Achievement{}

	for id := range gamesMap {
		eg.Go(func() error {
			rows, err := service.fetchAchievementsForGame(
				egCtx,
				client,
				steamUserID,
				id,
			)
			if err != nil {
				service.logger.WarnContext(egCtx, fmt.Sprintf(
					"failed to refresh achievements for %d; keeping existing data; error: %s",
					id,
					err,
				))
				return nil
			}

			mu.Lock()
			result[id] = rows
			mu.Unlock()
			return nil
		})
	}

	// All goroutines swallow their errors, so Wait never returns one.
	_ = eg.Wait()

	return result
}

func (service *SteamService) fetchAchievementsForGame(
	ctx context.Context,
	client steam.Client,
	steamUserID string,
	gameID int,
) ([]models.Achievement, error) {
	playerResp, err := client.GetPlayerAchievements(ctx, steamUserID, gameID)
	if err != nil {
		return nil, err
	}

	schemaResp, err := client.GetSchemaForGame(ctx, gameID)
	if err != nil {
		return nil, err
	}

	schemas := schemaResp.Game.AvailableGameStats.Achievements
	schemaMap := make(map[string]steam.AchievementSchema, len(schemas))
	for _, s := range schemas {
		schemaMap[s.Name] = s
	}

	globalPercents := service.fetchGlobalPercents(ctx, client, gameID)

	return buildAchievementRows(
		playerResp.PlayerStats.Achievements,
		schemas,
		schemaMap,
		globalPercents,
		gameID,
	), nil
}

// fetchGlobalPercents returns the global unlock percentages for a game. A
// failure is non-fatal: it logs and returns an empty map.
func (service *SteamService) fetchGlobalPercents(
	ctx context.Context,
	client steam.Client,
	gameID int,
) map[string]float64 {
	resp, err := client.GetGlobalAchievementPercentagesForApp(ctx, gameID)
	if err != nil {
		service.logger.WarnContext(ctx, fmt.Sprintf(
			"failed to fetch global percents for %d; error: %s", gameID, err,
		))
		return map[string]float64{}
	}

	percents := make(map[string]float64, len(resp.AchievementPercentages.Achievements))
	for _, a := range resp.AchievementPercentages.Achievements {
		if p, parseErr := a.Percent.Float64(); parseErr == nil {
			percents[a.Name] = p
		}
	}

	return percents
}

// buildAchievementRows merges the player's achievement state with the game
// schema (display name, description, icon) and global percentages into the rows
// stored in the database. When the player has no achievement state, the schema
// defines the (all unachieved) set.
func buildAchievementRows(
	playerAchievements []steam.Achievement,
	schemas []steam.AchievementSchema,
	schemaMap map[string]steam.AchievementSchema,
	globalPercents map[string]float64,
	gameID int,
) []models.Achievement {
	if len(playerAchievements) != 0 {
		rows := make([]models.Achievement, 0, len(playerAchievements))
		for _, a := range playerAchievements {
			schema := schemaMap[a.APIName]
			var unlockTime *time.Time
			if a.Achieved == 1 {
				value := time.Unix(a.UnlockTime, 0)
				unlockTime = &value
			}
			rows = append(rows, models.Achievement{
				Name:          a.APIName,
				DisplayName:   schema.DisplayName,
				Description:   schema.Description,
				IconURL:       schema.Icon,
				GameID:        gameID,
				Achieved:      a.Achieved == 1,
				UnlockTime:    unlockTime,
				GlobalPercent: percentPtr(globalPercents, a.APIName),
			})
		}
		return rows
	}

	rows := make([]models.Achievement, 0, len(schemas))
	for _, s := range schemas {
		rows = append(rows, models.Achievement{
			Name:          s.Name,
			DisplayName:   s.DisplayName,
			Description:   s.Description,
			IconURL:       s.Icon,
			GameID:        gameID,
			Achieved:      false,
			UnlockTime:    nil,
			GlobalPercent: percentPtr(globalPercents, s.Name),
		})
	}
	return rows
}

func percentPtr(globalPercents map[string]float64, name string) *float64 {
	if p, ok := globalPercents[name]; ok {
		return &p
	}
	return nil
}

// buildProgress recomputes the cumulative completion-rate graph from the freshly
// fetched achievements, keyed by the date each achievement was unlocked.
func buildProgress(fetched map[int][]models.Achievement) ([]string, []string) {
	totalAchievementsPerGame := make(map[int]int, len(fetched))
	for gameID, rows := range fetched {
		totalAchievementsPerGame[gameID] = len(rows)
	}

	grapher := helper.NewAchievementsGrapher(totalAchievementsPerGame)
	for gameID, rows := range fetched {
		for _, a := range rows {
			if a.Achieved && a.UnlockTime != nil {
				grapher.AddPoint(*a.UnlockTime, gameID)
			}
		}
	}

	return grapher.ToSlices()
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

// GetRecentlyActive returns the games the user unlocked achievements in over
// the last recentWindowDays, capped at recentGamesLimit, ordered most recent
// first. It powers the dashboard's "what you're working on" section.
func (service *SteamService) GetRecentlyActive(
	ctx context.Context,
	userID string,
) ([]models.RecentGame, error) {
	const recentWindowDays = 30
	const recentGamesLimit = 6
	since := time.Now().AddDate(0, 0, -recentWindowDays)
	return service.steam.GetRecentlyActiveGames(ctx, userID, since, recentGamesLimit)
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
