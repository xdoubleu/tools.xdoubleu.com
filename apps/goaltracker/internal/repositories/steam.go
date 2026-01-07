package repositories

import (
	"context"
	"time"

	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/jackc/pgx/v5"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
)

type SteamRepository struct {
	db postgres.DB
}

func (repo *SteamRepository) GetAllGames(
	ctx context.Context,
	userID string,
) ([]models.Game, error) {
	query := `
		SELECT id, name, is_delisted, completion_rate, contribution
		FROM goaltracker.steam_games
		WHERE user_id = $1 AND contribution != '0.0000'
	`

	rows, err := repo.db.Query(ctx, query, userID)
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
	games map[int]*models.Game,
	userID string,
) error {
	query := `
		INSERT INTO goaltracker.steam_games (id, user_id, name, is_delisted, 
		completion_rate, contribution)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id, user_id)
		DO UPDATE SET name = $3, is_delisted = $4, completion_rate = $5, contribution = $6
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
		)
	}

	err := repo.db.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}

func (repo *SteamRepository) GetAchievementsForGames(
	ctx context.Context,
	gameIDs []int,
	userID string,
) (map[int][]models.Achievement, error) {
	query := `
		SELECT game_id, name, achieved, unlock_time
		FROM goaltracker.steam_achievements
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
			&achievement.Achieved,
			&achievement.UnlockTime,
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

func (repo *SteamRepository) UpsertAchievements(
	ctx context.Context,
	achievements []steam.Achievement,
	userID string,
	gameID int,
) error {
	//nolint:exhaustruct //fields are optional
	tx, err := repo.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(
		ctx,
		"DELETE FROM goaltracker.steam_achievements WHERE game_id = $1 AND user_id = $2",
		gameID,
		userID,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	query := `
		INSERT INTO goaltracker.steam_achievements 
		(name, user_id, game_id, achieved, unlock_time)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name, user_id, game_id)
		DO UPDATE SET achieved = $4, unlock_time = $5
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for _, achievement := range achievements {
		var unlockTime *time.Time
		if achievement.Achieved == 1 {
			value := time.Unix(achievement.UnlockTime, 0)
			unlockTime = &value
		}
		b.Queue(
			query,
			achievement.APIName,
			userID,
			gameID,
			achievement.Achieved == 1,
			unlockTime,
		)
	}

	err = tx.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}

func (repo *SteamRepository) UpsertAchievementSchemas(
	ctx context.Context,
	achievementSchemas []steam.AchievementSchema,
	userID string,
	gameID int,
) error {
	query := `
		INSERT INTO goaltracker.steam_achievements (name, user_id, game_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (name, user_id, game_id)
		DO NOTHING
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for _, achievementSchema := range achievementSchemas {
		b.Queue(query, achievementSchema.Name, userID, gameID)
	}

	err := repo.db.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}
