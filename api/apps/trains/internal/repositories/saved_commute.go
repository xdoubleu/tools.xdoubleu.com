package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// SavedCommutesRepository accesses trains.saved_commutes; every method is
// scoped by user_id.
type SavedCommutesRepository struct {
	db postgres.DB
}

const uniqueViolationCode = "23505"

// savedCommuteSelect joins stored stop ids to station names; a stop no longer
// in the feed yields empty names rather than dropping the row.
const savedCommuteSelect = `
	SELECT sc.id, sc.user_id, sc.label, sc.origin_stop_id, sc.destination_stop_id,
	       sc.position, sc.created_at, sc.updated_at,
	       COALESCE(o.name_nl, ''), COALESCE(o.name_fr, ''), COALESCE(o.name_en, ''),
	       COALESCE(o.display_name, ''),
	       COALESCE(d.name_nl, ''), COALESCE(d.name_fr, ''), COALESCE(d.name_en, ''),
	       COALESCE(d.display_name, '')
	FROM trains.saved_commutes sc
	LEFT JOIN trains.stops o ON o.stop_id = sc.origin_stop_id
	LEFT JOIN trains.stops d ON d.stop_id = sc.destination_stop_id
`

func scanSavedCommute(row pgx.Row) (models.SavedCommute, error) {
	var sc models.SavedCommute
	err := row.Scan(
		&sc.ID, &sc.UserID, &sc.Label, &sc.OriginStopID, &sc.DestinationStopID,
		&sc.Position, &sc.CreatedAt, &sc.UpdatedAt,
		&sc.Origin.NameNL, &sc.Origin.NameFR, &sc.Origin.NameEN, &sc.Origin.DisplayName,
		&sc.Destination.NameNL, &sc.Destination.NameFR, &sc.Destination.NameEN,
		&sc.Destination.DisplayName,
	)
	sc.Origin.StopID = sc.OriginStopID
	sc.Destination.StopID = sc.DestinationStopID
	return sc, err
}

// ListByUser returns the user's saved commutes by position, then creation.
func (r *SavedCommutesRepository) ListByUser(
	ctx context.Context, userID string,
) ([]models.SavedCommute, error) {
	rows, err := r.db.Query(
		ctx,
		savedCommuteSelect+`
		WHERE sc.user_id = $1
		ORDER BY sc.position, sc.created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.SavedCommute
	for rows.Next() {
		sc, scanErr := scanSavedCommute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// Create appends a saved commute; a duplicate pair returns
// database.ErrResourceConflict.
func (r *SavedCommutesRepository) Create(
	ctx context.Context, sc models.SavedCommute,
) (models.SavedCommute, error) {
	row := r.db.QueryRow(
		ctx,
		`INSERT INTO trains.saved_commutes
			(user_id, label, origin_stop_id, destination_stop_id, position)
		 VALUES ($1, $2, $3, $4, (
			SELECT COALESCE(MAX(position) + 1, 0)
			FROM trains.saved_commutes WHERE user_id = $1
		 ))
		 RETURNING id`,
		sc.UserID, sc.Label, sc.OriginStopID, sc.DestinationStopID,
	)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return models.SavedCommute{}, database.ErrResourceConflict
		}
		return models.SavedCommute{}, err
	}
	return r.getByID(ctx, id, sc.UserID)
}

// Update changes label and position; returns database.ErrResourceNotFound if
// not owned by userID.
func (r *SavedCommutesRepository) Update(
	ctx context.Context, sc models.SavedCommute,
) (models.SavedCommute, error) {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE trains.saved_commutes
		 SET label = $3, position = $4, updated_at = now()
		 WHERE id = $1 AND user_id = $2`,
		sc.ID, sc.UserID, sc.Label, sc.Position,
	)
	if err != nil {
		return models.SavedCommute{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.SavedCommute{}, database.ErrResourceNotFound
	}
	return r.getByID(ctx, sc.ID, sc.UserID)
}

// Delete removes a commute; returns database.ErrResourceNotFound if missing
// or not owned.
func (r *SavedCommutesRepository) Delete(
	ctx context.Context, id uuid.UUID, userID string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM trains.saved_commutes WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

func (r *SavedCommutesRepository) getByID(
	ctx context.Context, id uuid.UUID, userID string,
) (models.SavedCommute, error) {
	sc, err := scanSavedCommute(r.db.QueryRow(
		ctx, savedCommuteSelect+`WHERE sc.id = $1 AND sc.user_id = $2`, id, userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.SavedCommute{}, database.ErrResourceNotFound
	}
	return sc, err
}
