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

// -------- WRITE (FIXED FOR YOUR STACK) --------

func (r *CalendarRepository) SaveFilterConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
	// --- FIX: never send NULL arrays to Postgres ---
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
		cfg.HideEventUIDs, // now always []string, never nil
		cfg.HolidayUIDs,   // now always []string, never nil
		string(seriesStr),
	)

	return err
}

// -------- READ (UNCHANGED LOGIC, JUST SIMPLER TYPES) --------

func (r *CalendarRepository) LoadFilterConfig(
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
