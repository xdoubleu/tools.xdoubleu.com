package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type AutomatedActionsRepository struct {
	db postgres.DB
}

func NewAutomatedActionsRepository(db postgres.DB) *AutomatedActionsRepository {
	return &AutomatedActionsRepository{db: db}
}

// Open records that routineName started and returns the id for Close.
func (r *AutomatedActionsRepository) Open(
	ctx context.Context,
	triggerSource, routineName string,
) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO global.automated_actions (trigger_source, routine_name)
		VALUES ($1, $2)
		RETURNING id
	`, triggerSource, routineName).Scan(&id)
	return id, err
}

// Close closes the row; empty prURL/errorText are stored as NULL.
func (r *AutomatedActionsRepository) Close(
	ctx context.Context,
	id int64,
	outcome, prURL, errorText string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE global.automated_actions
		SET finished_at = now(),
		    outcome = $2,
		    pr_url = NULLIF($3, ''),
		    error = NULLIF($4, '')
		WHERE id = $1
	`, id, outcome, prURL, errorText)
	return err
}

// staleActionSweepError is the error CloseStale records.
const staleActionSweepError = "stale open row auto-closed by api's " +
	"automated-action sweep: the routine that opened this row never closed it"

// CloseStale fails every open row fired before cutoff and returns their ids.
func (r *AutomatedActionsRepository) CloseStale(
	ctx context.Context,
	cutoff time.Time,
) ([]int64, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE global.automated_actions
		SET finished_at = now(),
		    outcome = 'failed',
		    error = $1
		WHERE finished_at IS NULL AND fired_at < $2
		RETURNING id
	`, staleActionSweepError, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// ListRecent returns runs since the given time, newest first.
func (r *AutomatedActionsRepository) ListRecent(
	ctx context.Context,
	since time.Time,
	limit int,
) ([]models.AutomatedAction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, fired_at, trigger_source, routine_name, finished_at,
		       COALESCE(outcome, ''), COALESCE(pr_url, ''), COALESCE(error, '')
		FROM global.automated_actions
		WHERE fired_at >= $1
		ORDER BY fired_at DESC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []models.AutomatedAction
	for rows.Next() {
		var a models.AutomatedAction
		if err = rows.Scan(
			&a.ID,
			&a.FiredAt,
			&a.TriggerSource,
			&a.RoutineName,
			&a.FinishedAt,
			&a.Outcome,
			&a.PRURL,
			&a.Error,
		); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}

	return actions, rows.Err()
}

// OldestOpenFiredAt returns the longest-open row's fired_at, or
// database.ErrResourceNotFound when none is open.
func (r *AutomatedActionsRepository) OldestOpenFiredAt(
	ctx context.Context,
) (time.Time, error) {
	var firedAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT fired_at
		FROM global.automated_actions
		WHERE finished_at IS NULL
		ORDER BY fired_at ASC
		LIMIT 1
	`).Scan(&firedAt)
	if err != nil {
		return time.Time{}, postgres.PgxErrorToHTTPError(err)
	}
	return firedAt, nil
}

// MostRecentOpenedAt returns fired_at of routineName's latest row (open or
// closed), or database.ErrResourceNotFound if it never opened one.
func (r *AutomatedActionsRepository) MostRecentOpenedAt(
	ctx context.Context,
	routineName string,
) (time.Time, error) {
	var firedAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT fired_at
		FROM global.automated_actions
		WHERE routine_name = $1
		ORDER BY fired_at DESC
		LIMIT 1
	`, routineName).Scan(&firedAt)
	if err != nil {
		return time.Time{}, postgres.PgxErrorToHTTPError(err)
	}
	return firedAt, nil
}
