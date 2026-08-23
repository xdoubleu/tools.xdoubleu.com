package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type LogsRepository struct {
	db postgres.DB
}

func NewLogsRepository(db postgres.DB) *LogsRepository {
	return &LogsRepository{db: db}
}

// Insert records one log entry. AttrsJSON may be nil.
func (r *LogsRepository) Insert(ctx context.Context, entry models.LogEntry) error {
	// Bind attrs as string, not []byte: under the simple query protocol
	// (used by the production connection pooler) a []byte is encoded as
	// bytea hex, which a JSONB column rejects.
	var attrs *string
	if entry.AttrsJSON != nil {
		s := string(entry.AttrsJSON)
		attrs = &s
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO global.log_entries (occurred_at, source, level, message, attrs)
		VALUES ($1, $2, $3, $4, $5)
	`, entry.OccurredAt, entry.Source, entry.Level, entry.Message, attrs)
	return err
}

// Query returns log entries at or after since, most recent first, optionally
// filtered by source and/or level. An empty source/level means "any".
func (r *LogsRepository) Query(
	ctx context.Context,
	since time.Time,
	source, level string,
) ([]models.LogEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT occurred_at, source, level, message, attrs
		FROM global.log_entries
		WHERE occurred_at >= $1
		  AND ($2 = '' OR source = $2)
		  AND ($3 = '' OR level = $3)
		ORDER BY occurred_at DESC
	`, since, source, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LogEntry
	for rows.Next() {
		var e models.LogEntry
		if err = rows.Scan(
			&e.OccurredAt, &e.Source, &e.Level, &e.Message, &e.AttrsJSON,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// PruneOlderThan deletes log entries older than cutoff.
func (r *LogsRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM global.log_entries WHERE occurred_at < $1", cutoff,
	)
	return err
}
