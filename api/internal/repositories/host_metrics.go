package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type HostMetricsRepository struct {
	db postgres.DB
}

func NewHostMetricsRepository(db postgres.DB) *HostMetricsRepository {
	return &HostMetricsRepository{db: db}
}

// Insert records one scraped sample.
func (r *HostMetricsRepository) Insert(
	ctx context.Context,
	sample models.HostMetricSample,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.host_metric_samples (
			sampled_at, cpu_percent, memory_percent, disk_percent
		) VALUES ($1, $2, $3, $4)
	`, sample.SampledAt, sample.CPUPercent, sample.MemoryPercent, sample.DiskPercent)
	return err
}

// Since returns every sample recorded at or after since, oldest first, for
// the history graphs.
func (r *HostMetricsRepository) Since(
	ctx context.Context,
	since time.Time,
) ([]models.HostMetricSample, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sampled_at, cpu_percent, memory_percent, disk_percent
		FROM global.host_metric_samples
		WHERE sampled_at >= $1
		ORDER BY sampled_at
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []models.HostMetricSample
	for rows.Next() {
		var s models.HostMetricSample
		if err = rows.Scan(
			&s.SampledAt, &s.CPUPercent, &s.MemoryPercent, &s.DiskPercent,
		); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}

	return samples, rows.Err()
}

// PruneOlderThan deletes samples older than cutoff.
func (r *HostMetricsRepository) PruneOlderThan(
	ctx context.Context,
	cutoff time.Time,
) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM global.host_metric_samples WHERE sampled_at < $1", cutoff,
	)
	return err
}
