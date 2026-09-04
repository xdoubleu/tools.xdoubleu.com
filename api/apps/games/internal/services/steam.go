package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/games/internal/helper"
	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/games/internal/repositories"
	"tools.xdoubleu.com/apps/games/pkg/steam"
)

type SteamService struct {
	logger        *slog.Logger
	clientFactory func(apiKey string) steam.Client
	steamAPIKey   string
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
	if service.steamAPIKey == "" || creds.SteamUserID == "" {
		service.logger.DebugContext(
			ctx,
			"steam not configured for user",
			"userID",
			userID,
		)
		return nil
	}

	client := service.clientFactory(service.steamAPIKey)

	gamesMap, err := service.buildGamesMap(ctx, client, creds.SteamUserID, userID)
	if err != nil {
		return err
	}

	fetched := service.fetchAchievements(ctx, client, creds.SteamUserID, gamesMap)

	// The progress graph must reflect the whole library, so include the stored
	// achievements of any game whose fetch failed this run (their data is kept,
	// not reset) instead of dropping them from the average.
	complete, err := service.completeAchievements(ctx, userID, gamesMap, fetched)
	if err != nil {
		return err
	}

	markCompletionAverageMembership(gamesMap, complete)
	averaged := averagedGames(gamesMap)

	for id := range gamesMap {
		rows, ok := fetched[id]
		if !ok {
			// Fetch failed/skipped for this game: keep its existing values.
			continue
		}
		gamesMap[id].SetCalculatedInfo(rows, len(averaged))
	}

	labels, values := buildProgress(averagedAchievements(complete, gamesMap))

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
		return service.progress.UpsertTx(ctx, tx, userID, labels, values)
	})
}

// completeAchievements returns the achievement rows used to compute derived
// progress: the freshly fetched rows plus, for any game whose fetch failed this
// run, the rows already stored in the database. This keeps the progress graph
// representative of the whole library rather than only this run's successes.
func (service *SteamService) completeAchievements(
	ctx context.Context,
	userID string,
	gamesMap map[int]*models.Game,
	fetched map[int][]models.Achievement,
) (map[int][]models.Achievement, error) {
	complete := make(map[int][]models.Achievement, len(gamesMap))
	for id, rows := range fetched {
		complete[id] = rows
	}

	missing := []int{}
	for id := range gamesMap {
		if _, ok := fetched[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return complete, nil
	}

	stored, err := service.steam.GetAchievementsForGames(ctx, missing, userID)
	if err != nil {
		return nil, err
	}
	for id, rows := range stored {
		complete[id] = rows
	}

	return complete, nil
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
	service.logger.DebugContext(
		ctx,
		fmt.Sprintf("fetched %d owned games", len(ownedResp.Response.Games)),
	)

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
			// set by markCompletionAverageMembership before persisting
			InCompletionAverage: false,
			ImageURL:            g.GetFullImgIconURL(),
			LastSyncedAt:        time.Time{}, // set by DB via now() in UpsertGames
			Favourite:           false,       // user-set; never written by UpsertGames
			LastPlayed:          rtimeLastPlayedToTime(g.RtimeLastPlayed),
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

// rtimeLastPlayedToTime converts Steam's rtime_last_played (Unix seconds, 0
// meaning never played) to a nullable timestamp for storage.
func rtimeLastPlayedToTime(rtimeLastPlayed int64) *time.Time {
	if rtimeLastPlayed == 0 {
		return nil
	}
	t := time.Unix(rtimeLastPlayed, 0).UTC()
	return &t
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

// markCompletionAverageMembership sets InCompletionAverage on every game. A
// game Steam still lists always counts. A delisted game counts too — the
// achievements earned there stay on the Steam profile — unless a listed game
// has taken its achievements over, which is what Valve did folding the
// Half-Life 2 episodes into Half-Life 2: counting the episode apps as well
// would put the same achievements into the average twice
// (docs/adr-0018-completion-average-population.md).
func markCompletionAverageMembership(
	gamesMap map[int]*models.Game,
	achievements map[int][]models.Achievement,
) {
	listed := make([]map[string]struct{}, 0, len(gamesMap))
	for id, game := range gamesMap {
		if game.IsDelisted {
			continue
		}
		if names := achievementNames(achievements[id]); len(names) > 0 {
			listed = append(listed, names)
		}
	}

	for id, game := range gamesMap {
		game.InCompletionAverage = !game.IsDelisted ||
			!supersededByListedGame(achievementNames(achievements[id]), listed)
	}
}

func achievementNames(rows []models.Achievement) map[string]struct{} {
	names := make(map[string]struct{}, len(rows))
	for _, a := range rows {
		names[a.Name] = struct{}{}
	}
	return names
}

// supersededByListedGame reports whether one single listed game carries every
// achievement in names. Containment has to be complete and within one game, so
// a partial overlap, or a set spread across several listed games, is not a
// takeover. Achievement API names are only unique per app, so a whole set of
// generic ones ("ACH_01") sitting inside an unrelated game would still read as
// one — unlikely enough to accept, given the alternative is a name-shape
// heuristic that would misjudge real takeovers.
func supersededByListedGame(
	names map[string]struct{},
	listed []map[string]struct{},
) bool {
	if len(names) == 0 {
		return false
	}
	for _, owned := range listed {
		if containsAll(owned, names) {
			return true
		}
	}
	return false
}

func containsAll(super, sub map[string]struct{}) bool {
	if len(super) < len(sub) {
		return false
	}
	for name := range sub {
		if _, ok := super[name]; !ok {
			return false
		}
	}
	return true
}

// averagedGames returns the subset of gamesMap taking part in the library-wide
// averages, as decided by markCompletionAverageMembership.
func averagedGames(gamesMap map[int]*models.Game) map[int]*models.Game {
	averaged := make(map[int]*models.Game, len(gamesMap))
	for id, game := range gamesMap {
		if game.InCompletionAverage {
			averaged[id] = game
		}
	}
	return averaged
}

// averagedAchievements drops the achievements of games outside the average so
// they take part in neither its numerator nor its denominator. A game absent
// from gamesMap is kept: it is being refreshed right now and its stored row
// simply has not been read back yet.
func averagedAchievements(
	achievements map[int][]models.Achievement,
	gamesMap map[int]*models.Game,
) map[int][]models.Achievement {
	averaged := make(map[int][]models.Achievement, len(achievements))
	for id, rows := range achievements {
		if game, ok := gamesMap[id]; ok && !game.InCompletionAverage {
			continue
		}
		averaged[id] = rows
	}
	return averaged
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

// GetAveragedGames returns the games taking part in the library-wide
// completion averages. See SteamRepository.GetAveragedGames.
func (service *SteamService) GetAveragedGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetAveragedGames(ctx, userID)
}

// GetDelisted returns the games Steam no longer lists in the user's library.
// See SteamRepository.GetDelisted for why they are reported separately.
func (service *SteamService) GetDelisted(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	return service.steam.GetDelisted(ctx, userID)
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

// SetFavourite flips the user-set favourite flag on a game.
func (service *SteamService) SetFavourite(
	ctx context.Context,
	userID string,
	gameID int,
	favourite bool,
) error {
	return service.steam.SetFavourite(ctx, userID, gameID, favourite)
}

// GetLastSyncedAt returns the most recent Steam sync across the user's
// library, or nil when never synced.
func (service *SteamService) GetLastSyncedAt(
	ctx context.Context,
	userID string,
) (*time.Time, error) {
	return service.steam.GetLastSyncedAt(ctx, userID)
}

// SyncGame refreshes a single game's achievements from Steam and persists the
// updated rows, its completion rate, and the library-wide progress graph
// (same recompute as SyncUser) so the dashboard's total completion rate stays
// in sync with this game's refreshed achievements.
func (service *SteamService) SyncGame(
	ctx context.Context,
	userID string,
	gameID int,
) error {
	creds, err := service.integrations.Get(ctx, userID)
	if err != nil {
		return err
	}
	if service.steamAPIKey == "" || creds.SteamUserID == "" {
		service.logger.DebugContext(
			ctx,
			"steam not configured for user",
			"userID",
			userID,
		)
		return nil
	}

	client := service.clientFactory(service.steamAPIKey)

	rows, err := service.fetchAchievementsForGame(
		ctx, client, creds.SteamUserID, gameID,
	)
	if err != nil {
		return err
	}

	allGames, err := service.steam.GetAllGames(ctx, userID)
	if err != nil {
		return err
	}

	game, err := service.steam.GetGameByID(ctx, gameID, userID)
	if err != nil {
		return err
	}

	// Recompute the library-wide progress graph so the dashboard's total
	// completion rate reflects this game's refreshed achievements (same as
	// SyncUser). Other games contribute their stored achievements.
	gamesMap := make(map[int]*models.Game, len(allGames))
	for i := range allGames {
		g := allGames[i]
		gamesMap[g.ID] = &g
	}

	fetched := map[int][]models.Achievement{gameID: rows}
	complete, err := service.completeAchievements(ctx, userID, gamesMap, fetched)
	if err != nil {
		return err
	}

	markCompletionAverageMembership(gamesMap, complete)
	game.InCompletionAverage = gamesMap[gameID].InCompletionAverage
	game.SetCalculatedInfo(rows, len(averagedGames(gamesMap)))

	labels, values := buildProgress(averagedAchievements(complete, gamesMap))

	return service.steam.WithTx(ctx, func(tx pgx.Tx) error {
		if errIn := service.steam.ReplaceAchievements(
			ctx, tx, userID, gameID, rows,
		); errIn != nil {
			return errIn
		}
		if errIn := service.steam.UpsertGames(
			ctx, tx, map[int]*models.Game{gameID: game}, userID,
		); errIn != nil {
			return errIn
		}
		return service.progress.UpsertTx(ctx, tx, userID, labels, values)
	})
}

// GetRecentlyActive returns the games the user most recently played, capped
// at recentGamesLimit and ordered most recent first. It powers the
// dashboard's "recently active" section.
func (service *SteamService) GetRecentlyActive(
	ctx context.Context,
	userID string,
) ([]models.RecentGame, error) {
	const recentGamesLimit = 5
	return service.steam.GetRecentlyActiveGames(ctx, userID, recentGamesLimit)
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
