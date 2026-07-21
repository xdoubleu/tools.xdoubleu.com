package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/games/internal/models"
)

type SteamRepository struct {
	db postgres.DB
}

// WithTx runs fn inside a single transaction, committing on success and rolling
// back on any error so a Steam refresh applies atomically.
func (repo *SteamRepository) WithTx(
	ctx context.Context,
	fn func(tx pgx.Tx) error,
) error {
	//nolint:exhaustruct //default tx options
	tx, err := repo.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}

// queryGames runs a games query and scans every row into a models.Game. All the
// list endpoints share the same column projection, so they delegate the row
// handling here and differ only in their WHERE/ORDER BY clauses.
func (repo *SteamRepository) queryGames(
	ctx context.Context,
	query string,
	args ...any,
) ([]models.Game, error) {
	rows, err := repo.db.Query(ctx, query, args...)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	games := []models.Game{}
	for rows.Next() {
		var game models.Game

		err = rows.Scan(
			&game.ID,
			&game.Name,
			&game.IsDelisted,
			&game.CompletionRate,
			&game.Contribution,
			&game.Playtime,
			&game.ImageURL,
			&game.LastSyncedAt,
			&game.Favourite,
			&game.LastPlayed,
		)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return games, nil
}

func (repo *SteamRepository) GetAllGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	query := `
		SELECT id, name, is_delisted, completion_rate, contribution,
		       playtime_forever, image_url, last_synced_at, favourite, last_played
		FROM games.steam_games
		WHERE user_id = $1
	`

	return repo.queryGames(ctx, query, userID)
}

func (repo *SteamRepository) GetBacklog(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	query := `
		SELECT sg.id, sg.name, sg.is_delisted, sg.completion_rate,
		       sg.contribution, sg.playtime_forever, sg.image_url, sg.last_synced_at,
		       sg.favourite, sg.last_played
		FROM games.steam_games sg
		WHERE sg.user_id = $1
		    AND CAST(sg.completion_rate AS FLOAT) = 0
		    AND sg.is_delisted = false
		    AND EXISTS (
		        SELECT 1 FROM games.steam_achievements sa
		        WHERE sa.game_id = sg.id AND sa.user_id = $1
		    )
		ORDER BY sg.name
	`

	return repo.queryGames(ctx, query, userID)
}

func (repo *SteamRepository) GetInProgress(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	query := `
		SELECT sg.id, sg.name, sg.is_delisted, sg.completion_rate,
		       sg.contribution, sg.playtime_forever, sg.image_url, sg.last_synced_at,
		       sg.favourite, sg.last_played
		FROM games.steam_games sg
		WHERE sg.user_id = $1
		    AND CAST(sg.completion_rate AS FLOAT) > 0
		    AND sg.is_delisted = false
		    AND CAST(sg.completion_rate AS FLOAT) < 100
		    AND EXISTS (
		        SELECT 1 FROM games.steam_achievements sa
		        WHERE sa.game_id = sg.id AND sa.user_id = $1
		    )
		ORDER BY CAST(sg.completion_rate AS FLOAT) ASC, sg.name
	`

	return repo.queryGames(ctx, query, userID)
}

func (repo *SteamRepository) GetCompleted(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	query := `
		SELECT sg.id, sg.name, sg.is_delisted, sg.completion_rate,
		       sg.contribution, sg.playtime_forever, sg.image_url, sg.last_synced_at,
		       sg.favourite, sg.last_played
		FROM games.steam_games sg
		WHERE sg.user_id = $1
		    AND sg.is_delisted = false
		    AND CAST(sg.completion_rate AS FLOAT) >= 100
		    AND EXISTS (
		        SELECT 1 FROM games.steam_achievements sa
		        WHERE sa.game_id = sg.id AND sa.user_id = $1
		    )
		ORDER BY CAST(sg.completion_rate AS FLOAT) ASC, sg.name
	`

	return repo.queryGames(ctx, query, userID)
}

// GetRecentlyActiveGames returns the games the user most recently played,
// ordered by last_played descending and capped at limit. Games never played
// (last_played IS NULL) are excluded.
func (repo *SteamRepository) GetRecentlyActiveGames(
	ctx context.Context,
	userID string,
	limit int,
) ([]models.RecentGame, error) {
	query := `
		SELECT id, name, completion_rate, image_url, playtime_forever, last_played
		FROM games.steam_games
		WHERE user_id = $1
		    AND last_played IS NOT NULL
		    AND is_delisted = false
		ORDER BY last_played DESC
		LIMIT $2
	`

	rows, err := repo.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	games := []models.RecentGame{}
	for rows.Next() {
		var game models.RecentGame

		err = rows.Scan(
			&game.ID,
			&game.Name,
			&game.CompletionRate,
			&game.ImageURL,
			&game.Playtime,
			&game.LastPlayed,
		)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return games, nil
}

func (repo *SteamRepository) UpsertGames(
	ctx context.Context,
	q Querier,
	games map[int]*models.Game,
	userID string,
) error {
	if q == nil {
		q = repo.db
	}

	// favourite is deliberately absent from both column lists: it is
	// user-set state and must survive every sync (new rows default FALSE).
	query := `
		INSERT INTO games.steam_games
		    (id, user_id, name, is_delisted, completion_rate, contribution,
		     playtime_forever, image_url, last_synced_at, last_played)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), $9)
		ON CONFLICT (id, user_id)
		DO UPDATE SET name = $3, is_delisted = $4, completion_rate = $5,
		              contribution = $6, playtime_forever = $7, image_url = $8,
		              last_synced_at = now(), last_played = $9
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for _, game := range games {
		b.Queue(
			query,
			game.ID,
			userID,
			game.Name,
			game.IsDelisted,
			game.CompletionRate,
			game.Contribution,
			game.Playtime,
			game.ImageURL,
			game.LastPlayed,
		)
	}

	err := q.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}

// SetFavourite flips the user-set favourite flag on a game. Returns
// database.ErrResourceNotFound when the game is not in the user's library.
func (repo *SteamRepository) SetFavourite(
	ctx context.Context,
	userID string,
	gameID int,
	favourite bool,
) error {
	tag, err := repo.db.Exec(ctx, `
		UPDATE games.steam_games
		SET favourite = $3
		WHERE id = $1 AND user_id = $2
	`, gameID, userID, favourite)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// GetLastSyncedAt returns the most recent Steam sync across the user's
// library, or nil when no game has ever been synced.
func (repo *SteamRepository) GetLastSyncedAt(
	ctx context.Context,
	userID string,
) (*time.Time, error) {
	var lastSynced *time.Time
	err := repo.db.QueryRow(ctx, `
		SELECT max(last_synced_at) FROM games.steam_games WHERE user_id = $1
	`, userID).Scan(&lastSynced)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return lastSynced, nil
}

func (repo *SteamRepository) GetAchievementsForGames(
	ctx context.Context,
	gameIDs []int,
	userID string,
) (map[int][]models.Achievement, error) {
	query := `
		SELECT game_id, name, display_name, description, icon_url,
		       achieved, unlock_time, global_percent
		FROM games.steam_achievements
		WHERE game_id = ANY($1) AND user_id = $2
	`

	rows, err := repo.db.Query(ctx, query, gameIDs, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	achievements := map[int][]models.Achievement{}
	for rows.Next() {
		//nolint:exhaustruct //other fields are defined later
		achievement := models.Achievement{}

		err = rows.Scan(
			&achievement.GameID,
			&achievement.Name,
			&achievement.DisplayName,
			&achievement.Description,
			&achievement.IconURL,
			&achievement.Achieved,
			&achievement.UnlockTime,
			&achievement.GlobalPercent,
		)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		achievements[achievement.GameID] = append(
			achievements[achievement.GameID],
			achievement,
		)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return achievements, nil
}

func (repo *SteamRepository) GetGameByID(
	ctx context.Context,
	gameID int,
	userID string,
) (*models.Game, error) {
	query := `
		SELECT id, name, is_delisted, completion_rate, contribution,
		       playtime_forever, image_url, last_synced_at, favourite, last_played
		FROM games.steam_games
		WHERE id = $1 AND user_id = $2
	`

	var game models.Game
	err := repo.db.QueryRow(ctx, query, gameID, userID).Scan(
		&game.ID,
		&game.Name,
		&game.IsDelisted,
		&game.CompletionRate,
		&game.Contribution,
		&game.Playtime,
		&game.ImageURL,
		&game.LastSyncedAt,
		&game.Favourite,
		&game.LastPlayed,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &game, nil
}

// ReplaceAchievements replaces all stored achievements for a game with the given
// rows: it deletes the existing rows and inserts the fresh set. It runs on the
// supplied Querier (pass a transaction to make a refresh atomic; pass nil to use
// the repository connection).
func (repo *SteamRepository) ReplaceAchievements(
	ctx context.Context,
	q Querier,
	userID string,
	gameID int,
	achievements []models.Achievement,
) error {
	if q == nil {
		q = repo.db
	}

	_, err := q.Exec(
		ctx,
		"DELETE FROM games.steam_achievements WHERE game_id = $1 AND user_id = $2",
		gameID,
		userID,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	query := `
		INSERT INTO games.steam_achievements
		    (name, display_name, description, icon_url, user_id, game_id,
		     achieved, unlock_time, global_percent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (name, user_id, game_id)
		DO UPDATE SET display_name = $2, description = $3, icon_url = $4,
		              achieved = $7, unlock_time = $8, global_percent = $9
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for _, achievement := range achievements {
		b.Queue(
			query,
			achievement.Name,
			achievement.DisplayName,
			achievement.Description,
			achievement.IconURL,
			userID,
			gameID,
			achievement.Achieved,
			achievement.UnlockTime,
			achievement.GlobalPercent,
		)
	}

	if err = q.SendBatch(ctx, b).Close(); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}
