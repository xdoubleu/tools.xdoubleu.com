package repositories

import (
	"context"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/models"
)

type UsageRepository struct {
	db postgres.DB
}

func NewUsageRepository(db postgres.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

// Flush adds the accumulated counts to global.usage_daily.
func (r *UsageRepository) Flush(
	ctx context.Context,
	entries []models.UsageEntry,
) error {
	for _, e := range entries {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO global.usage_daily (day, app, endpoint, count)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (day, app, endpoint)
			DO UPDATE SET count = usage_daily.count + EXCLUDED.count
		`, e.Day, e.App, e.Endpoint, e.Count); err != nil {
			return err
		}
	}
	return nil
}

func (r *UsageRepository) GetDaily(
	ctx context.Context,
	since time.Time,
) ([]models.UsageEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT day, app, endpoint, count
		FROM global.usage_daily
		WHERE day >= $1
		ORDER BY day, app, endpoint
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.UsageEntry
	for rows.Next() {
		var e models.UsageEntry
		if err = rows.Scan(&e.Day, &e.App, &e.Endpoint, &e.Count); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

func (r *UsageRepository) PruneOlderThan(
	ctx context.Context,
	cutoff time.Time,
) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM global.usage_daily WHERE day < $1", cutoff,
	)
	return err
}
