package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xdoubleu/essentia/v2/pkg/database"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type GoalRepository struct {
	db postgres.DB
}

func (repo *GoalRepository) GetAll(
	ctx context.Context,
	userID string,
) ([]models.Goal, error) {
	query := `
		SELECT id, name, type_id, source_id, 
		( select value
		from goaltracker.progress
		where goaltracker.progress.type_id = goals.type_id
		order by date desc
		limit 1
		) as progress, target_value, 
		state_id, is_linked, period, due_time, "order", 
		config
		FROM goaltracker.goals
		WHERE user_id = $1
		ORDER BY "order" asc
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	goals := []models.Goal{}
	for rows.Next() {
		//nolint:exhaustruct //other fields are initialized later
		goal := models.Goal{}

		err = rows.Scan(
			&goal.ID,
			&goal.Name,
			&goal.TypeID,
			&goal.SourceID,
			&goal.Progress,
			&goal.TargetValue,
			&goal.StateID,
			&goal.IsLinked,
			&goal.Period,
			&goal.DueTime,
			&goal.Order,
			&goal.Config,
		)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		goals = append(goals, goal)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return goals, nil
}

func (repo *GoalRepository) GetByID(
	ctx context.Context,
	id string,
	userID string,
) (*models.Goal, error) {
	query := `
		SELECT name, type_id, source_id, target_value, 
		state_id, is_linked, period, due_time, "order", config
		FROM goaltracker.goals
		WHERE id = $1 AND user_id = $2
	`

	//nolint:exhaustruct //other fields are optional
	goal := models.Goal{
		ID: id,
	}
	err := repo.db.QueryRow(
		ctx,
		query,
		id,
		userID).Scan(
		&goal.Name,
		&goal.TypeID,
		&goal.SourceID,
		&goal.TargetValue,
		&goal.StateID,
		&goal.IsLinked,
		&goal.Period,
		&goal.DueTime,
		&goal.Order,
		&goal.Config,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &goal, nil
}

func (repo *GoalRepository) GetByTypeID(
	ctx context.Context,
	id int64,
	userID string,
) ([]models.Goal, error) {
	query := `
		SELECT id, name, source_id, target_value, state_id,
		is_linked, period, due_time, "order", config
		FROM goaltracker.goals
		WHERE type_id = $1 AND user_id = $2
	`

	rows, err := repo.db.Query(ctx, query, id, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	goals := []models.Goal{}
	for rows.Next() {
		//nolint:exhaustruct //other fields are assigned later
		goal := models.Goal{
			TypeID: &id,
		}

		err = rows.Scan(
			&goal.ID,
			&goal.Name,
			&goal.SourceID,
			&goal.TargetValue,
			&goal.StateID,
			&goal.IsLinked,
			&goal.Period,
			&goal.DueTime,
			&goal.Order,
			&goal.Config,
		)

		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		goals = append(goals, goal)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return goals, nil
}

func (repo *GoalRepository) Upsert(
	ctx context.Context,
	id string,
	userID string,
	name string,
	stateID string,
	due *todoist.Due,
	order int,
) (*models.Goal, error) {
	query := `
		INSERT INTO goaltracker.goals (id, user_id, name, state_id, period, due_time, "order")
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id, user_id)
		DO UPDATE SET name = $3, state_id = $4, 
		period = $5, due_time = $6, "order" = $7
		RETURNING id
	`

	var dueTime *time.Time
	var period *models.Period
	if due != nil {
		dueTime = &due.Date.Time

		if due.IsRecurring {
			period = models.TodoistDueStringToPeriod(due.String)
		}
	}

	//nolint:exhaustruct //other fields are optional
	goal := models.Goal{
		ID:      id,
		Name:    name,
		StateID: stateID,
		Period:  period,
		DueTime: dueTime,
		Order:   order,
		Config:  nil,
	}

	err := repo.db.QueryRow(
		ctx,
		query,
		id,
		userID,
		name,
		stateID,
		period,
		dueTime,
		order,
	).Scan(&goal.ID)

	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &goal, nil
}

func (repo *GoalRepository) Link(
	ctx context.Context,
	goal *models.Goal,
	userID string,
	linkGoalDto dtos.LinkGoalDto,
) error {
	query := `
		UPDATE goaltracker.goals
		SET is_linked = true, target_value = $3, type_id = $4, source_id = $5, config = $6
		WHERE id = $1 AND user_id = $2
	`

	config := map[string]string{}

	if linkGoalDto.Tag != nil {
		config["tag"] = *linkGoalDto.Tag
	}

	var serializedConfig *string
	if len(config) > 0 {
		bytesConfig, err := json.Marshal(config)
		if err != nil {
			return err
		}
		t := string(bytesConfig)
		serializedConfig = &t
	}

	result, err := repo.db.Exec(
		ctx,
		query,
		goal.ID,
		userID,
		linkGoalDto.TargetValue,
		linkGoalDto.TypeID,
		models.SourcesTypeIDMap[linkGoalDto.TypeID].ID,
		serializedConfig,
	)

	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return database.ErrResourceNotFound
	}

	goal.Config = config

	return nil
}

func (repo *GoalRepository) Unlink(
	ctx context.Context,
	goal models.Goal,
	userID string,
) error {
	query := `
		UPDATE goaltracker.goals
		SET is_linked = false, target_value = null, type_id = null, config = null
		WHERE id = $1 AND user_id = $2
	`

	result, err := repo.db.Exec(
		ctx,
		query,
		goal.ID,
		userID,
	)

	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return database.ErrResourceNotFound
	}

	return nil
}

func (repo *GoalRepository) Delete(
	ctx context.Context,
	goal *models.Goal,
	userID string,
) error {
	query := `
		DELETE FROM goaltracker.goals
		WHERE id = $1 AND user_id = $2
	`

	result, err := repo.db.Exec(ctx, query, goal.ID, userID)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return database.ErrResourceNotFound
	}

	return nil
}
