package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/repositories"
)

func clearAutomatedActions(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.automated_actions")
	require.NoError(t, err)
}

func TestAutomatedActionsOpenAndClose(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	id, err := repo.Open(t.Context(), "schedule", "dependabot-triage")
	require.NoError(t, err)
	assert.Positive(t, id)

	runs, err := repo.ListRecent(t.Context(), time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, id, runs[0].ID)
	assert.Equal(t, "schedule", runs[0].TriggerSource)
	assert.Equal(t, "dependabot-triage", runs[0].RoutineName)
	assert.Nil(t, runs[0].FinishedAt)
	assert.Empty(t, runs[0].Outcome)
	assert.Empty(t, runs[0].PRURL)
	assert.Empty(t, runs[0].Error)

	require.NoError(t, repo.Close(
		t.Context(), id, "succeeded", "https://github.com/o/r/pull/1", "",
	))

	runs, err = repo.ListRecent(t.Context(), time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.NotNil(t, runs[0].FinishedAt)
	assert.Equal(t, "succeeded", runs[0].Outcome)
	assert.Equal(t, "https://github.com/o/r/pull/1", runs[0].PRURL)
	assert.Empty(t, runs[0].Error)
}

func TestAutomatedActionsCloseFailed(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	id, err := repo.Open(t.Context(), "manual", "restart-worker")
	require.NoError(t, err)

	require.NoError(t, repo.Close(t.Context(), id, "failed", "", "boom"))

	runs, err := repo.ListRecent(t.Context(), time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "failed", runs[0].Outcome)
	assert.Equal(t, "boom", runs[0].Error)
	assert.Empty(t, runs[0].PRURL)
}

func TestAutomatedActionsListRecentOrdersNewestFirstAndRespectsWindow(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.automated_actions
			(fired_at, trigger_source, routine_name)
		VALUES (now() - INTERVAL '2 days', 'schedule', 'old-routine')
	`)
	require.NoError(t, err)

	id, err := repo.Open(t.Context(), "api", "new-routine")
	require.NoError(t, err)

	runs, err := repo.ListRecent(t.Context(), time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, id, runs[0].ID)

	runs, err = repo.ListRecent(t.Context(), time.Now().Add(-72*time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "new-routine", runs[0].RoutineName)
	assert.Equal(t, "old-routine", runs[1].RoutineName)
}

func TestAutomatedActionsOldestOpenFiredAtNoneOpen(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	_, err := repo.OldestOpenFiredAt(t.Context())
	require.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestAutomatedActionsOldestOpenFiredAtReturnsOldestStillOpen(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	// A closed row should be ignored even though it fired earliest.
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.automated_actions
			(fired_at, finished_at, trigger_source, routine_name, outcome)
		VALUES (now() - INTERVAL '3 hours', now(), 'schedule', 'closed-routine',
		        'succeeded')
	`)
	require.NoError(t, err)

	_, err = testDB.Exec(t.Context(), `
		INSERT INTO global.automated_actions
			(fired_at, trigger_source, routine_name)
		VALUES (now() - INTERVAL '2 hours', 'schedule', 'older-open-routine')
	`)
	require.NoError(t, err)

	_, err = repo.Open(t.Context(), "api", "newer-open-routine")
	require.NoError(t, err)

	firedAt, err := repo.OldestOpenFiredAt(t.Context())
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(-2*time.Hour), firedAt, time.Minute)
}

func TestAutomatedActionsMostRecentOpenedAtNeverOpened(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	_, err := repo.MostRecentOpenedAt(t.Context(), "never-run-routine")
	require.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestAutomatedActionsMostRecentOpenedAtReturnsNewestForRoutine(t *testing.T) {
	clearAutomatedActions(t)
	repo := repositories.NewAutomatedActionsRepository(testDB)

	// An older run of the same routine should be superseded by the newer one.
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.automated_actions
			(fired_at, finished_at, trigger_source, routine_name, outcome)
		VALUES (now() - INTERVAL '2 days', now() - INTERVAL '2 days',
		        'schedule', 'nightly-maintenance-sweep', 'succeeded')
	`)
	require.NoError(t, err)

	// A different routine's row should never be picked up.
	_, err = repo.Open(t.Context(), "schedule", "ready-issues-executor")
	require.NoError(t, err)

	_, err = testDB.Exec(t.Context(), `
		INSERT INTO global.automated_actions
			(fired_at, finished_at, trigger_source, routine_name, outcome)
		VALUES (now() - INTERVAL '1 hour', now() - INTERVAL '1 hour',
		        'schedule', 'nightly-maintenance-sweep', 'succeeded')
	`)
	require.NoError(t, err)

	firedAt, err := repo.MostRecentOpenedAt(
		t.Context(), "nightly-maintenance-sweep",
	)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(-time.Hour), firedAt, time.Minute)
}
