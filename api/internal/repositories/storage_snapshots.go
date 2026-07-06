package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/models"
)

// storageSnapshotRetention bounds global.storage_snapshots (~13 months of
// daily scans); older rows are pruned on insert.
const storageSnapshotRetention = 400 * 24 * time.Hour

type StorageSnapshotsRepository struct {
	db postgres.DB
}

func NewStorageSnapshotsRepository(db postgres.DB) *StorageSnapshotsRepository {
	return &StorageSnapshotsRepository{db: db}
}

func (r *StorageSnapshotsRepository) Insert(
	ctx context.Context,
	snap models.StorageSnapshot,
) error {
	breakdown, err := json.Marshal(snap.PrefixBreakdown)
	if err != nil {
		return err
	}
	// Bind as string, not []byte: under the simple query protocol (used by the
	// production connection pooler) a []byte is encoded as bytea hex, which a
	// JSONB column rejects with "invalid input syntax for type json".
	_, err = r.db.Exec(ctx, `
		INSERT INTO global.storage_snapshots (
			scanned_at, total_size_bytes, object_count,
			orphan_size_bytes, orphan_count,
			stale_upload_size_bytes, stale_upload_count, prefix_breakdown
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		snap.ScannedAt, snap.TotalSizeBytes, snap.ObjectCount,
		snap.OrphanSizeBytes, snap.OrphanCount,
		snap.StaleUploadSizeBytes, snap.StaleUploadCount, string(breakdown),
	)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`DELETE FROM global.storage_snapshots WHERE scanned_at < $1`,
		time.Now().Add(-storageSnapshotRetention),
	)
	return err
}

func (r *StorageSnapshotsRepository) Latest(
	ctx context.Context,
) (*models.StorageSnapshot, error) {
	snap, err := scanSnapshot(r.db.QueryRow(ctx, `
		SELECT scanned_at, total_size_bytes, object_count,
		       orphan_size_bytes, orphan_count,
		       stale_upload_size_bytes, stale_upload_count, prefix_breakdown
		FROM global.storage_snapshots
		ORDER BY scanned_at DESC
		LIMIT 1
	`))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return snap, nil
}

// History returns snapshots taken since the given time, oldest first, for the
// trend chart.
func (r *StorageSnapshotsRepository) History(
	ctx context.Context,
	since time.Time,
) ([]models.StorageSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT scanned_at, total_size_bytes, object_count,
		       orphan_size_bytes, orphan_count,
		       stale_upload_size_bytes, stale_upload_count, prefix_breakdown
		FROM global.storage_snapshots
		WHERE scanned_at >= $1
		ORDER BY scanned_at
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []models.StorageSnapshot
	for rows.Next() {
		snap, scanErr := scanSnapshot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		snaps = append(snaps, *snap)
	}

	return snaps, rows.Err()
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row rowScanner) (*models.StorageSnapshot, error) {
	var (
		snap      models.StorageSnapshot
		breakdown []byte
	)
	if err := row.Scan(
		&snap.ScannedAt, &snap.TotalSizeBytes, &snap.ObjectCount,
		&snap.OrphanSizeBytes, &snap.OrphanCount,
		&snap.StaleUploadSizeBytes, &snap.StaleUploadCount, &breakdown,
	); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(breakdown, &snap.PrefixBreakdown); err != nil {
		return nil, err
	}
	return &snap, nil
}
