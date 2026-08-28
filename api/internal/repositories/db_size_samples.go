package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type DBSizeSamplesRepository struct {
	db postgres.DB
}

func NewDBSizeSamplesRepository(db postgres.DB) *DBSizeSamplesRepository {
	return &DBSizeSamplesRepository{db: db}
}

// InsertBatch records one sampled_at snapshot's worth of per-table sizes.
func (r *DBSizeSamplesRepository) InsertBatch(
	ctx context.Context,
	sampledAt time.Time,
	samples []models.TableSizeSample,
) error {
	for _, s := range samples {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO global.db_size_samples (
				sampled_at, schema_name, table_name, size_bytes
			) VALUES ($1, $2, $3, $4)
		`, sampledAt, s.SchemaName, s.TableName, s.SizeBytes); err != nil {
			return err
		}
	}
	return nil
}

// PruneOlderThan deletes samples older than cutoff.
func (r *DBSizeSamplesRepository) PruneOlderThan(
	ctx context.Context,
	cutoff time.Time,
) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM global.db_size_samples WHERE sampled_at < $1", cutoff,
	)
	return err
}

// Growth compares each table's most recent sampled size against its
// earliest size recorded at or after since, returning the fastest-growing
// tables first. A table sampled only once within the window (no growth to
// compute yet) is excluded rather than reported as a false zero delta.
func (r *DBSizeSamplesRepository) Growth(
	ctx context.Context,
	since time.Time,
) ([]models.TableSizeGrowth, error) {
	rows, err := r.db.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (schema_name, table_name)
				schema_name, table_name, size_bytes, sampled_at
			FROM global.db_size_samples
			ORDER BY schema_name, table_name, sampled_at DESC
		), earliest AS (
			SELECT DISTINCT ON (schema_name, table_name)
				schema_name, table_name, size_bytes, sampled_at
			FROM global.db_size_samples
			WHERE sampled_at >= $1
			ORDER BY schema_name, table_name, sampled_at ASC
		)
		SELECT
			l.schema_name, l.table_name, l.size_bytes, e.size_bytes,
			l.size_bytes - e.size_bytes AS delta_bytes,
			CASE WHEN e.size_bytes = 0 THEN 0
				ELSE (l.size_bytes - e.size_bytes)::double precision / e.size_bytes
			END AS pct_change
		FROM latest l
		JOIN earliest e
			ON e.schema_name = l.schema_name AND e.table_name = l.table_name
		WHERE l.sampled_at > e.sampled_at
		ORDER BY delta_bytes DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var growth []models.TableSizeGrowth
	for rows.Next() {
		var g models.TableSizeGrowth
		if err = rows.Scan(
			&g.SchemaName, &g.TableName, &g.CurrentSizeBytes, &g.EarliestSizeBytes,
			&g.DeltaBytes, &g.PctChange,
		); err != nil {
			return nil, err
		}
		growth = append(growth, g)
	}

	return growth, rows.Err()
}
