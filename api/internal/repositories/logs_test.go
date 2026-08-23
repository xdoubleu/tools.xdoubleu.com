package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
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

	all, err := repo.Query(t.Context(), now.Add(-time.Hour), "", "")
	require.NoError(t, err)
	require.Len(t, all, 2)
	// Most recent first.
	assert.Equal(t, "web oops", all[0].Message)

	apiOnly, err := repo.Query(t.Context(), now.Add(-time.Hour), "api", "")
	require.NoError(t, err)
	require.Len(t, apiOnly, 1)
	assert.Equal(t, "api hello", apiOnly[0].Message)

	errOnly, err := repo.Query(t.Context(), now.Add(-time.Hour), "", "error")
	require.NoError(t, err)
	require.Len(t, errOnly, 1)
	assert.Equal(t, "web oops", errOnly[0].Message)
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

	entries, err := repo.Query(t.Context(), time.Now().AddDate(0, 0, -90), "", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "fresh", entries[0].Message)
}
