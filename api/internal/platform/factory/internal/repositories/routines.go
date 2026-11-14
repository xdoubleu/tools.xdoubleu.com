package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type RoutinesRepository struct {
	db postgres.DB
}

const routineColumns = `name, repo_id, enabled, interval_minutes, created_at`

func scanRoutine(row rowScanner) (*models.Routine, error) {
	var r models.Routine
	err := row.Scan(&r.Name, &r.RepoID, &r.Enabled, &r.IntervalMinutes, &r.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &r, nil
}

func (r *RoutinesRepository) rows(
	ctx context.Context,
	query string,
) ([]models.Routine, error) {
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Routine{}
	for rows.Next() {
		routine, scanErr := scanRoutine(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *routine)
	}
	return out, rows.Err()
}

// List returns every routine by name.
func (r *RoutinesRepository) List(ctx context.Context) ([]models.Routine, error) {
	return r.rows(ctx,
		`SELECT `+routineColumns+` FROM factory.routines ORDER BY name`)
}

// Get returns one routine; an unknown name is not found.
func (r *RoutinesRepository) Get(
	ctx context.Context,
	name string,
) (*models.Routine, error) {
	return scanRoutine(r.db.QueryRow(ctx,
		`SELECT `+routineColumns+` FROM factory.routines WHERE name = $1`, name))
}

// Save creates a routine or changes its repo, flag and interval.
func (r *RoutinesRepository) Save(
	ctx context.Context,
	routine models.Routine,
) (*models.Routine, error) {
	return scanRoutine(r.db.QueryRow(ctx, `
		INSERT INTO factory.routines (name, repo_id, enabled, interval_minutes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET
			repo_id = EXCLUDED.repo_id, enabled = EXCLUDED.enabled,
			interval_minutes = EXCLUDED.interval_minutes
		RETURNING `+routineColumns,
		routine.Name, routine.RepoID, routine.Enabled, routine.IntervalMinutes))
}

// Due returns the enabled scheduled routines with no job queued or running and
// none created within their interval.
func (r *RoutinesRepository) Due(ctx context.Context) ([]models.Routine, error) {
	return r.rows(ctx, `
		SELECT `+routineColumns+` FROM factory.routines r
		WHERE enabled AND interval_minutes > 0 AND NOT EXISTS (
			SELECT 1 FROM factory.jobs j WHERE j.routine = r.name AND (
				j.status IN ('queued', 'running')
				OR j.created_at > now() - make_interval(mins => r.interval_minutes)
			)
		)
		ORDER BY name`)
}
