package repositories

import (
	"context"

	"github.com/xdoubleu/essentia/v2/pkg/database"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

type StateRepository struct {
	db postgres.DB
}

func (repo *StateRepository) GetAll(
	ctx context.Context,
	userID string,
) ([]models.State, error) {
	query := `
		SELECT id, name, "order"
		FROM goaltracker.states
		WHERE user_id = $1
		ORDER BY "order"
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	states := []models.State{}
	for rows.Next() {
		//nolint:exhaustruct //other fields are initialized later
		state := models.State{}

		err = rows.Scan(
			&state.ID,
			&state.Name,
			&state.Order,
		)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		states = append(states, state)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return states, nil
}

func (repo *StateRepository) Upsert(
	ctx context.Context,
	id string,
	userID string,
	name string,
	order int,
) (*models.State, error) {
	query := `
		INSERT INTO goaltracker.states (id, user_id, name, "order")
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id, user_id)
		DO UPDATE SET name = $3, "order" = $4
		RETURNING id
	`

	state := models.State{
		ID:    id,
		Name:  name,
		Order: order,
	}

	err := repo.db.QueryRow(
		ctx,
		query,
		id,
		userID,
		name,
		order,
	).Scan(&state.ID)

	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &state, nil
}

func (repo *StateRepository) Delete(
	ctx context.Context,
	state *models.State,
	userID string,
) error {
	query := `
		DELETE FROM goaltracker.states
		WHERE id = $1 AND user_id = $2
	`

	result, err := repo.db.Exec(ctx, query, state.ID, userID)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return database.ErrResourceNotFound
	}

	return nil
}
