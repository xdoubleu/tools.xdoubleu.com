package repositories

import (
	"context"
	"encoding/json"

	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

type CalendarRepository struct {
	db postgres.DB
}

// =====================================================
// CREATE / UPSERT
// =====================================================

// Explicit alias (nicer API).
func (r *CalendarRepository) UpsertFilterConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
	// Ensure Postgres never receives NULL arrays/maps
	if cfg.HideEventUIDs == nil {
		cfg.HideEventUIDs = []string{}
	}
	if cfg.HolidayUIDs == nil {
		cfg.HolidayUIDs = []string{}
	}
	if cfg.HideSeries == nil {
		cfg.HideSeries = map[string]bool{}
	}

	seriesStr, _ := json.Marshal(cfg.HideSeries)

	_, err := r.db.Exec(ctx, `
		INSERT INTO icsproxy.feeds
		(token, source_url, hide_event_uids, holiday_uids, hide_series)
		VALUES ($1,$2,$3,$4,$5::jsonb)
		ON CONFLICT (token) DO UPDATE SET
		  source_url=$2,
		  hide_event_uids=$3,
		  holiday_uids=$4,
		  hide_series=$5::jsonb
	`,
		cfg.Token,
		cfg.SourceURL,
		cfg.HideEventUIDs,
		cfg.HolidayUIDs,
		string(seriesStr),
	)

	return err
}

// =====================================================
// READ ONE
// =====================================================

// Clearer public name.
func (r *CalendarRepository) GetFilterConfig(
	ctx context.Context,
	token string,
) (models.FilterConfig, bool) {
	var cfg models.FilterConfig
	var seriesJSON []byte

	err := r.db.QueryRow(ctx, `
		SELECT token, source_url, hide_event_uids, holiday_uids, hide_series
		FROM icsproxy.feeds
		WHERE token=$1
	`, token).Scan(
		&cfg.Token,
		&cfg.SourceURL,
		&cfg.HideEventUIDs,
		&cfg.HolidayUIDs,
		&seriesJSON,
	)

	if err != nil {
		return cfg, false
	}

	if len(seriesJSON) > 0 {
		_ = json.Unmarshal(seriesJSON, &cfg.HideSeries)
	} else {
		cfg.HideSeries = map[string]bool{}
	}

	return cfg, true
}

// =====================================================
// READ ALL
// =====================================================

// Returns full configs (useful for edit pages).
func (r *CalendarRepository) ListFilterConfigs(
	ctx context.Context,
) ([]models.FilterConfig, error) {
	rows, err := r.db.Query(ctx, `
		SELECT token, source_url, hide_event_uids, holiday_uids, hide_series
		FROM icsproxy.feeds
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []models.FilterConfig

	for rows.Next() {
		var cfg models.FilterConfig
		var seriesJSON []byte

		if err = rows.Scan(
			&cfg.Token,
			&cfg.SourceURL,
			&cfg.HideEventUIDs,
			&cfg.HolidayUIDs,
			&seriesJSON,
		); err != nil {
			return nil, err
		}

		if len(seriesJSON) > 0 {
			_ = json.Unmarshal(seriesJSON, &cfg.HideSeries)
		} else {
			cfg.HideSeries = map[string]bool{}
		}

		configs = append(configs, cfg)
	}

	return configs, rows.Err()
}

// Lightweight list for homepage (recommended).
type FilterSummary struct {
	Token     string
	SourceURL string
}

func (r *CalendarRepository) ListFilterSummaries(
	ctx context.Context,
) ([]FilterSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT token, source_url
		FROM icsproxy.feeds
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FilterSummary

	for rows.Next() {
		var s FilterSummary
		if err = rows.Scan(&s.Token, &s.SourceURL); err != nil {
			return nil, err
		}
		out = append(out, s)
	}

	return out, rows.Err()
}

// =====================================================
// DELETE
// =====================================================

func (r *CalendarRepository) DeleteFilterConfig(
	ctx context.Context,
	token string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM icsproxy.feeds
		WHERE token = $1
	`, token)

	return err
}
