package repositories_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

func clearLogs(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.log_entries")
	require.NoError(t, err)
}

func TestLogsInsertAndQuery(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now()

	require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
		OccurredAt: now, Source: "api", Level: "info", Message: "api hello",
		AttrsJSON: []byte(`{"a":1}`),
	}))
	require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
		OccurredAt: now.Add(time.Second), Source: "web", Level: "error",
		Message: "web oops", AttrsJSON: nil,
	}))

	all, err := repo.Query(t.Context(), sinceFilter(now.Add(-time.Hour)))
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "web oops", all[0].Message)

	apiFilter := sinceFilter(now.Add(-time.Hour))
	apiFilter.Source = "api"
	apiOnly, err := repo.Query(t.Context(), apiFilter)
	require.NoError(t, err)
	require.Len(t, apiOnly, 1)
	assert.Equal(t, "api hello", apiOnly[0].Message)

	errFilter := sinceFilter(now.Add(-time.Hour))
	errFilter.MinLevel = "error"
	errOnly, err := repo.Query(t.Context(), errFilter)
	require.NoError(t, err)
	require.Len(t, errOnly, 1)
	assert.Equal(t, "web oops", errOnly[0].Message)
}

func TestLogsInsertBatch(t *testing.T) {
	for name, mode := range map[string]pgx.QueryExecMode{
		"extended": pgx.QueryExecModeCacheStatement,
		"simple":   pgx.QueryExecModeSimpleProtocol,
	} {
		t.Run(name, func(t *testing.T) {
			pgxCfg, err := pgxpool.ParseConfig(testhelper.NewTestConfig().DBDsn)
			require.NoError(t, err)
			pgxCfg.ConnConfig.DefaultQueryExecMode = mode
			pool, err := pgxpool.NewWithConfig(t.Context(), pgxCfg)
			require.NoError(t, err)
			defer pool.Close()

			clearLogs(t)
			repo := repositories.NewLogsRepository(pool)
			now := time.Now()
			require.NoError(t, repo.InsertBatch(t.Context(), []models.LogEntry{
				{
					OccurredAt: now, Source: "api", Level: "INFO",
					Message: "first", AttrsJSON: []byte(`{"a":1}`),
				},
				{
					OccurredAt: now.Add(time.Second), Source: "api", Level: "WARN",
					Message: "second", AttrsJSON: nil,
				},
			}))

			entries, err := repo.Query(t.Context(), sinceFilter(now.Add(-time.Hour)))
			require.NoError(t, err)
			assert.Equal(t, []string{"second", "first"}, messages(entries))
			assert.JSONEq(t, `{"a":1}`, string(entries[1].AttrsJSON))
		})
	}
}

func TestLogsInsertBatch_FailsWholeBatch(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now()

	err := repo.InsertBatch(t.Context(), []models.LogEntry{
		{OccurredAt: now, Source: "api", Level: "INFO", Message: "ok", AttrsJSON: nil},
		{
			OccurredAt: now, Source: "api", Level: "INFO", Message: "bad",
			AttrsJSON: []byte(`not json`),
		},
	})

	// The log writer retries row by row only on a server-side rejection.
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	entries, err := repo.Query(t.Context(), sinceFilter(now.Add(-time.Hour)))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func sinceFilter(since time.Time) repositories.LogsFilter {
	return repositories.LogsFilter{
		Since: since, Until: time.Time{}, Source: "", MinLevel: "",
		Contains: "", Limit: 100,
	}
}

func insertLog(
	t *testing.T,
	repo *repositories.LogsRepository,
	at time.Time,
	level, message string,
	attrs []byte,
) {
	t.Helper()
	require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
		OccurredAt: at, Source: "api", Level: level, Message: message,
		AttrsJSON: attrs,
	}))
}

func messages(entries []models.LogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Message
	}
	return out
}

func TestLogsQuery_MinLevelIsAtLeast(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now()
	// api stores slog's uppercase levels, web ingest lowercase.
	insertLog(t, repo, now, "DEBUG", "debug", nil)
	insertLog(t, repo, now.Add(time.Second), "info", "info", nil)
	insertLog(t, repo, now.Add(2*time.Second), "WARN", "warn", nil)
	insertLog(t, repo, now.Add(3*time.Second), "error", "error", nil)
	insertLog(t, repo, now.Add(4*time.Second), "ERROR+2", "error+2", nil)

	for _, tc := range []struct {
		minLevel string
		want     []string
	}{
		{"", []string{"error+2", "error", "warn", "info", "debug"}},
		{"debug", []string{"error+2", "error", "warn", "info", "debug"}},
		{"INFO", []string{"error+2", "error", "warn", "info"}},
		{"Warn", []string{"error+2", "error", "warn"}},
		{"ERROR", []string{"error+2", "error"}},
	} {
		t.Run(tc.minLevel, func(t *testing.T) {
			f := sinceFilter(now.Add(-time.Hour))
			f.MinLevel = tc.minLevel
			entries, err := repo.Query(t.Context(), f)
			require.NoError(t, err)
			assert.Equal(t, tc.want, messages(entries))
		})
	}
}

func TestLogsQuery_UnknownMinLevel(t *testing.T) {
	repo := repositories.NewLogsRepository(testDB)
	f := sinceFilter(time.Now().Add(-time.Hour))
	f.MinLevel = "fatal"
	_, err := repo.Query(t.Context(), f)
	require.ErrorIs(t, err, repositories.ErrUnknownLogLevel)
}

func TestLogsQuery_UntilIsExclusiveUpperBound(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now().Truncate(time.Second)
	for i := range 3 {
		insertLog(t, repo, now.Add(time.Duration(i)*time.Minute), "INFO",
			[]string{"oldest", "middle", "newest"}[i], nil)
	}

	f := sinceFilter(now.Add(-time.Hour))
	f.Until = now.Add(2 * time.Minute)
	f.Limit = 1
	entries, err := repo.Query(t.Context(), f)
	require.NoError(t, err)
	assert.Equal(t, []string{"middle"}, messages(entries))
}

func TestLogsQuery_ContainsMatchesMessageAndAttrs(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now()
	insertLog(t, repo, now, "INFO", "request",
		[]byte(`{"path":"/books/kobo/abc/v1/library/sync"}`))
	insertLog(t, repo, now.Add(time.Second), "INFO", "Kobo cover fetched", nil)
	insertLog(t, repo, now.Add(2*time.Second), "INFO", "request",
		[]byte(`{"path":"/metrics"}`))

	f := sinceFilter(now.Add(-time.Hour))
	f.Contains = "KOBO"
	entries, err := repo.Query(t.Context(), f)
	require.NoError(t, err)
	assert.Equal(t, []string{"Kobo cover fetched", "request"}, messages(entries))
}

func TestLogsPruneOlderThan(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)

	old := time.Now().AddDate(0, 0, -60)
	require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
		OccurredAt: old, Source: "api", Level: "info", Message: "old",
		AttrsJSON: nil,
	}))
	require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
		OccurredAt: time.Now(), Source: "api", Level: "info", Message: "fresh",
		AttrsJSON: nil,
	}))

	require.NoError(
		t, repo.PruneOlderThan(t.Context(), time.Now().AddDate(0, 0, -30)),
	)

	entries, err := repo.Query(t.Context(), sinceFilter(time.Now().AddDate(0, 0, -90)))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "fresh", entries[0].Message)
}

func TestLogsQuery_RespectsLimit(t *testing.T) {
	clearLogs(t)
	repo := repositories.NewLogsRepository(testDB)
	now := time.Now()
	for i := range 3 {
		require.NoError(t, repo.Insert(t.Context(), models.LogEntry{
			OccurredAt: now.Add(time.Duration(i) * time.Second), Source: "api",
			Level: "info", Message: "entry", AttrsJSON: nil,
		}))
	}

	f := sinceFilter(now.Add(-time.Hour))
	f.Limit = 2
	entries, err := repo.Query(t.Context(), f)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// Production goes through a transaction-mode pooler (simple protocol).
func TestLogsQuery_AllFiltersSimpleProtocol(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pgxCfg, err := pgxpool.ParseConfig(cfg.DBDsn)
	require.NoError(t, err)
	pgxCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(t.Context(), pgxCfg)
	require.NoError(t, err)
	defer pool.Close()

	clearLogs(t)
	repo := repositories.NewLogsRepository(pool)
	now := time.Now().Truncate(time.Second)
	insertLog(t, repo, now, "WARN", "kobo match", nil)
	insertLog(t, repo, now, "INFO", "kobo too quiet", nil)
	insertLog(t, repo, now.Add(time.Hour), "ERROR", "kobo too late", nil)

	entries, err := repo.Query(t.Context(), repositories.LogsFilter{
		Since: now.Add(-time.Hour), Until: now.Add(time.Minute), Source: "api",
		MinLevel: "warn", Contains: "kobo", Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"kobo match"}, messages(entries))
}
