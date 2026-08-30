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

// History returns the total sampled database size per snapshot batch, oldest
// first, for batches taken at or after since. InsertBatch writes every table of
// one snapshot under a single sampled_at, so grouping by it yields exactly one
// row per snapshot.
func (r *DBSizeSamplesRepository) History(
	ctx context.Context,
	since time.Time,
) ([]models.DBSizeSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sampled_at, SUM(size_bytes)::bigint
		FROM global.db_size_samples
		WHERE sampled_at >= $1
		GROUP BY sampled_at
		ORDER BY sampled_at ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.DBSizeSnapshot
	for rows.Next() {
		var s models.DBSizeSnapshot
		if err = rows.Scan(&s.SampledAt, &s.TotalSizeBytes); err != nil {
			return nil, err
		}
		history = append(history, s)
	}

	return history, rows.Err()
}

// PerTableHistory returns every (day, schema, table) row on or after since,
// oldest first — the flat series GetDatabaseSizeHistory returns as-is; the
// client pivots and selects which series to plot, summing a schema's tables
// client-side for the schema-level view rather than a second query.
func (r *DBSizeSamplesRepository) PerTableHistory(
	ctx context.Context,
	since time.Time,
) ([]models.TableSizeHistoryPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sampled_at::date, schema_name, table_name, size_bytes
		FROM global.db_size_samples
		WHERE sampled_at >= $1
		ORDER BY sampled_at
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TableSizeHistoryPoint
	for rows.Next() {
		var p models.TableSizeHistoryPoint
		if err = rows.Scan(
			&p.Day, &p.SchemaName, &p.TableName, &p.SizeBytes,
		); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, rows.Err()
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
