package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

// AlertStatesRepository reads/writes global.alert_states, the breach/
// recovery state jobs.ThresholdAlertJob evaluates against (issue #1283).
type AlertStatesRepository struct {
	db postgres.DB
}

func NewAlertStatesRepository(db postgres.DB) *AlertStatesRepository {
	return &AlertStatesRepository{db: db}
}

// Get returns rule's current state, or nil if the rule has never been
// evaluated yet (no row written).
func (r *AlertStatesRepository) Get(
	ctx context.Context,
	ruleKey string,
) (*models.AlertState, error) {
	var s models.AlertState
	err := r.db.QueryRow(ctx, `
		SELECT rule_key, breaching, since, last_notified_at, current_value, threshold
		FROM global.alert_states
		WHERE rule_key = $1
	`, ruleKey).Scan(
		&s.RuleKey, &s.Breaching, &s.Since, &s.LastNotifiedAt,
		&s.CurrentValue, &s.Threshold,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // absence is a valid "never evaluated" state
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Upsert writes rule's latest evaluation. Called on every job run for every
// enabled rule, not just on a breach/recovery transition, so CurrentValue
// always reflects the latest sample even between transitions.
func (r *AlertStatesRepository) Upsert(
	ctx context.Context,
	s models.AlertState,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.alert_states (
			rule_key, breaching, since, last_notified_at, current_value, threshold
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (rule_key) DO UPDATE SET
			breaching = $2,
			since = $3,
			last_notified_at = $4,
			current_value = $5,
			threshold = $6
	`, s.RuleKey, s.Breaching, s.Since, s.LastNotifiedAt, s.CurrentValue, s.Threshold)
	return err
}

// List returns every rule's current state, alphabetically by key — used by
// get_alert_states, which reports the state of a rule even when it has
// never breached.
func (r *AlertStatesRepository) List(
	ctx context.Context,
) ([]models.AlertState, error) {
	rows, err := r.db.Query(ctx, `
		SELECT rule_key, breaching, since, last_notified_at, current_value, threshold
		FROM global.alert_states
		ORDER BY rule_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []models.AlertState
	for rows.Next() {
		var s models.AlertState
		if scanErr := rows.Scan(
			&s.RuleKey, &s.Breaching, &s.Since, &s.LastNotifiedAt,
			&s.CurrentValue, &s.Threshold,
		); scanErr != nil {
			return nil, scanErr
		}
		states = append(states, s)
	}
	return states, rows.Err()
}
