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

// Open records that routineName has started, fired by triggerSource, and
// returns the row id Close needs to close it out.
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

// Close closes out the row id refers to, recording how the run ended.
// prURL/errorText are optional — NULLIF blanks them to NULL rather than
// storing an empty string.
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

// ListRecent returns the most recent runs since the given time, newest
// first.
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

// OldestOpenFiredAt returns the fired_at timestamp of the longest-open
// automated_actions row (finished_at IS NULL) — the one a stalled-routine
// alert should key off, since it is the run that has been running the
// longest without closing out. Returns database.ErrResourceNotFound when no
// row is currently open.
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
