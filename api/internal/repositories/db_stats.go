package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type DBStatsRepository struct {
	db postgres.DB
}

func NewDBStatsRepository(db postgres.DB) *DBStatsRepository {
	return &DBStatsRepository{db: db}
}

// TotalSize returns the on-disk size of the current database in bytes.
func (r *DBStatsRepository) TotalSize(ctx context.Context) (int64, error) {
	var size int64
	err := r.db.QueryRow(
		ctx,
		"SELECT pg_database_size(current_database())",
	).Scan(&size)
	return size, err
}

// SchemaSizes returns the on-disk size and table count of every non-system
// schema, largest first.
func (r *DBStatsRepository) SchemaSizes(
	ctx context.Context,
) ([]models.SchemaStats, error) {
	rows, err := r.db.Query(ctx, `
		SELECT n.nspname,
		       COALESCE(SUM(pg_total_relation_size(c.oid)), 0)::bigint,
		       COUNT(*) FILTER (WHERE c.relkind = 'r')
		FROM pg_namespace n
		LEFT JOIN pg_class c
		       ON c.relnamespace = n.oid AND c.relkind IN ('r', 'i', 'm')
		WHERE n.nspname NOT LIKE 'pg_%'
		  AND n.nspname <> 'information_schema'
		GROUP BY n.nspname
		ORDER BY 2 DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.SchemaStats
	for rows.Next() {
		var s models.SchemaStats
		if err = rows.Scan(&s.Name, &s.SizeBytes, &s.TableCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// TableSizes returns the live on-disk size of every table in every
// non-system schema, largest first — the per-table scrape
// DBSizeSnapshotJob (issue #1282) persists daily into
// global.db_size_samples.
func (r *DBStatsRepository) TableSizes(
	ctx context.Context,
) ([]models.TableSizeSample, error) {
	rows, err := r.db.Query(ctx, `
		SELECT n.nspname, c.relname, pg_total_relation_size(c.oid)::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT LIKE 'pg_%'
		  AND n.nspname <> 'information_schema'
		ORDER BY 3 DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []models.TableSizeSample
	for rows.Next() {
		var s models.TableSizeSample
		if err = rows.Scan(&s.SchemaName, &s.TableName, &s.SizeBytes); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}

	return samples, rows.Err()
}
