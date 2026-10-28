package repositories

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type LogsRepository struct {
	db postgres.DB
}

func NewLogsRepository(db postgres.DB) *LogsRepository {
	return &LogsRepository{db: db}
}

const insertLogEntrySQL = `
	INSERT INTO global.log_entries (occurred_at, source, level, message, attrs)
	VALUES ($1, $2, $3, $4, $5)
`

// logEntryArgs binds AttrsJSON as a string: under the simple protocol
// (pooler) []byte is sent as bytea hex, which JSONB rejects.
func logEntryArgs(entry models.LogEntry) []any {
	var attrs *string
	if entry.AttrsJSON != nil {
		s := string(entry.AttrsJSON)
		attrs = &s
	}
	return []any{entry.OccurredAt, entry.Source, entry.Level, entry.Message, attrs}
}

// Insert records one log entry. AttrsJSON may be nil.
func (r *LogsRepository) Insert(ctx context.Context, entry models.LogEntry) error {
	_, err := r.db.Exec(ctx, insertLogEntrySQL, logEntryArgs(entry)...)
	return err
}

// InsertBatch records entries in one round trip, all or none.
func (r *LogsRepository) InsertBatch(
	ctx context.Context,
	entries []models.LogEntry,
) error {
	//nolint:exhaustruct //QueuedQueries is populated by Queue below
	batch := &pgx.Batch{}
	for _, entry := range entries {
		batch.Queue(insertLogEntrySQL, logEntryArgs(entry)...)
	}
	return r.db.SendBatch(ctx, batch).Close()
}

// ErrUnknownLogLevel is returned for a MinLevel outside DEBUG/INFO/WARN/ERROR.
var ErrUnknownLogLevel = errors.New("unknown log level")

// LogsFilter narrows Query. Empty strings match any; a zero Until means no
// upper bound.
type LogsFilter struct {
	Since    time.Time
	Until    time.Time
	Source   string
	MinLevel string // at least this level, case-insensitive
	Contains string // case-insensitive substring of message or attrs JSON
	Limit    int
}

// logLevelRank orders levels by severity from 1; 0 means no level filter.
func logLevelRank(level string) (int, error) {
	if level == "" {
		return 0, nil
	}
	i := slices.Index(
		[]string{"DEBUG", "INFO", "WARN", "ERROR"}, strings.ToUpper(level),
	)
	if i < 0 {
		return 0, fmt.Errorf("%w: %q", ErrUnknownLogLevel, level)
	}
	return i + 1, nil
}

// Query returns up to f.Limit entries in [f.Since, f.Until), newest first.
func (r *LogsRepository) Query(
	ctx context.Context,
	f LogsFilter,
) ([]models.LogEntry, error) {
	minRank, err := logLevelRank(f.MinLevel)
	if err != nil {
		return nil, err
	}
	var until *time.Time
	if !f.Until.IsZero() {
		until = &f.Until
	}

	rows, err := r.db.Query(ctx, `
		SELECT occurred_at, source, level, message, attrs
		FROM global.log_entries
		WHERE occurred_at >= $1
		  AND ($2::timestamptz IS NULL OR occurred_at < $2)
		  AND ($3 = '' OR source = $3)
		  AND CASE
		        WHEN upper(level) LIKE 'ERROR%' THEN 4
		        WHEN upper(level) LIKE 'WARN%' THEN 3
		        WHEN upper(level) LIKE 'INFO%' THEN 2
		        ELSE 1
		      END >= $4
		  AND ($5 = ''
		       OR strpos(lower(message), lower($5)) > 0
		       OR strpos(lower(coalesce(attrs::text, '')), lower($5)) > 0)
		ORDER BY occurred_at DESC
		LIMIT $6
	`, f.Since, until, f.Source, minRank, f.Contains, f.Limit)
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
