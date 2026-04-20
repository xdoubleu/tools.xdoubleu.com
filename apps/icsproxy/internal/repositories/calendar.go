package repositories

import (
	"context"
	"encoding/json"

	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

type CalendarRepository struct {
	db postgres.DB
}

// =====================================================
// CREATE / UPSERT
// =====================================================

func (r *CalendarRepository) UpsertFilterConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
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
		(token, user_id, source_url, hide_event_uids, holiday_uids, hide_series)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		ON CONFLICT (token) DO UPDATE SET
		  source_url=$3,
		  hide_event_uids=$4,
		  holiday_uids=$5,
		  hide_series=$6::jsonb
		WHERE icsproxy.feeds.user_id = EXCLUDED.user_id
	`,
		cfg.Token,
		cfg.UserID,
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

func (r *CalendarRepository) GetFilterConfig(
	ctx context.Context,
	token string,
) (models.FilterConfig, bool) {
	var cfg models.FilterConfig
	var seriesJSON []byte

	err := r.db.QueryRow(ctx, `
		SELECT token, user_id, source_url, hide_event_uids, holiday_uids, hide_series
		FROM icsproxy.feeds
		WHERE token=$1
	`, token).Scan(
		&cfg.Token,
		&cfg.UserID,
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
// READ ALL (user-scoped)
// =====================================================

func (r *CalendarRepository) ListFilterConfigs(
	ctx context.Context,
	userID string,
) ([]models.FilterConfig, error) {
	rows, err := r.db.Query(ctx, `
		SELECT token, user_id, source_url, hide_event_uids, holiday_uids, hide_series
		FROM icsproxy.feeds
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
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
			&cfg.UserID,
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

// =====================================================
// READ SUMMARIES (user-scoped)
// =====================================================

type FilterSummary struct {
	Token     string
	SourceURL string
}

func (r *CalendarRepository) ListFilterSummaries(
	ctx context.Context,
	userID string,
) ([]FilterSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT token, source_url
		FROM icsproxy.feeds
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
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
// DELETE (user-scoped)
// =====================================================

func (r *CalendarRepository) DeleteFilterConfig(
	ctx context.Context,
	token string,
	userID string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM icsproxy.feeds
		WHERE token = $1
		  AND user_id = $2
	`, token, userID)

	return err
}
