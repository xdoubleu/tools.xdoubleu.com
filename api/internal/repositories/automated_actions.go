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

// Close closes the row; empty prURL/errorText and nil metrics are stored as
// NULL.
func (r *AutomatedActionsRepository) Close(
	ctx context.Context,
	id int64,
	outcome, prURL, errorText string,
	metrics *models.RunMetrics,
) error {
	m := runMetricsParams(metrics)
	_, err := r.db.Exec(ctx, `
		UPDATE global.automated_actions
		SET finished_at = now(),
		    outcome = $2,
		    pr_url = NULLIF($3, ''),
		    error = NULLIF($4, ''),
		    requests = $5,
		    input_tokens = $6,
		    output_tokens = $7,
		    reasoning_tokens = $8,
		    cache_read_tokens = $9,
		    cost_usd = $10,
		    duration_seconds = $11,
		    tool_calls = $12,
		    tool_errors = $13,
		    repeated_tool_calls = $14
		WHERE id = $1
	`, append([]any{id, outcome, prURL, errorText}, m...)...)
	return err
}

// runMetricsParams returns Close's metric parameters, all nil for nil m.
func runMetricsParams(m *models.RunMetrics) []any {
	if m == nil {
		return make([]any, runMetricsColumns)
	}
	return []any{
		m.Requests, m.InputTokens, m.OutputTokens, m.ReasoningTokens,
		m.CacheReadTokens, m.CostUSD, m.DurationSeconds, m.ToolCalls,
		m.ToolErrors, m.RepeatedToolCalls,
	}
}

const runMetricsColumns = 10

// runMetricsSelect lists the metric columns in scanRunMetrics' order.
const runMetricsSelect = `requests, input_tokens, output_tokens,
	reasoning_tokens, cache_read_tokens, cost_usd, duration_seconds,
	tool_calls, tool_errors, repeated_tool_calls`

// runMetricsScan holds nullable metric columns until they're known present.
type runMetricsScan struct {
	requests, toolCalls, toolErrors, repeated *int32
	input, output, reasoning, cacheRead       *int64
	cost, duration                            *float64
}

func (s *runMetricsScan) dest() []any {
	return []any{
		&s.requests, &s.input, &s.output, &s.reasoning, &s.cacheRead,
		&s.cost, &s.duration, &s.toolCalls, &s.toolErrors, &s.repeated,
	}
}

// metrics is nil when the row was closed without metrics; requests is
// always set when any metric is.
func (s *runMetricsScan) metrics() *models.RunMetrics {
	if s.requests == nil {
		return nil
	}
	return &models.RunMetrics{
		Requests:          *s.requests,
		InputTokens:       deref(s.input),
		OutputTokens:      deref(s.output),
		ReasoningTokens:   deref(s.reasoning),
		CacheReadTokens:   deref(s.cacheRead),
		CostUSD:           deref(s.cost),
		DurationSeconds:   deref(s.duration),
		ToolCalls:         deref(s.toolCalls),
		ToolErrors:        deref(s.toolErrors),
		RepeatedToolCalls: deref(s.repeated),
	}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
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
		       COALESCE(outcome, ''), COALESCE(pr_url, ''), COALESCE(error, ''),
		       `+runMetricsSelect+`
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
		var m runMetricsScan
		if err = rows.Scan(append([]any{
			&a.ID,
			&a.FiredAt,
			&a.TriggerSource,
			&a.RoutineName,
			&a.FinishedAt,
			&a.Outcome,
			&a.PRURL,
			&a.Error,
		}, m.dest()...)...); err != nil {
			return nil, err
		}
		a.Metrics = m.metrics()
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

// LatestRunMetrics returns each routine's most recent run closed with
// metrics since the given time, keyed by routine name.
func (r *AutomatedActionsRepository) LatestRunMetrics(
	ctx context.Context,
	since time.Time,
) (map[string]models.RunMetrics, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (routine_name) routine_name, `+runMetricsSelect+`
		FROM global.automated_actions
		WHERE fired_at >= $1 AND requests IS NOT NULL
		ORDER BY routine_name, fired_at DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	latest := map[string]models.RunMetrics{}
	for rows.Next() {
		var routine string
		var m runMetricsScan
		if err = rows.Scan(append([]any{&routine}, m.dest()...)...); err != nil {
			return nil, err
		}
		latest[routine] = *m.metrics()
	}

	return latest, rows.Err()
}
