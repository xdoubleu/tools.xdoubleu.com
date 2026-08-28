package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearAlertStates(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.alert_states")
	require.NoError(t, err)
}

func TestAlertStatesGetMissingReturnsNil(t *testing.T) {
	clearAlertStates(t)
	repo := repositories.NewAlertStatesRepository(testDB)

	state, err := repo.Get(t.Context(), "does_not_exist")
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestAlertStatesUpsertInsertsThenUpdates(t *testing.T) {
	clearAlertStates(t)
	repo := repositories.NewAlertStatesRepository(testDB)

	since := time.Now().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, repo.Upsert(t.Context(), models.AlertState{
		RuleKey: "host_cpu_high", Breaching: true, Since: &since,
		LastNotifiedAt: &since, CurrentValue: 92.5, Threshold: 80,
	}))

	state, err := repo.Get(t.Context(), "host_cpu_high")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.Breaching)
	require.NotNil(t, state.Since)
	assert.WithinDuration(t, since, *state.Since, time.Second)
	assert.InDelta(t, 92.5, state.CurrentValue, 0.001)
	assert.InDelta(t, 80, state.Threshold, 0.001)

	// Recovery: breaching flips back, since/last_notified_at cleared/updated
	// -- the re-arm this table exists for (issue #1283).
	notifiedAt := time.Now().Truncate(time.Second)
	require.NoError(t, repo.Upsert(t.Context(), models.AlertState{
		RuleKey: "host_cpu_high", Breaching: false, Since: nil,
		LastNotifiedAt: &notifiedAt, CurrentValue: 40, Threshold: 80,
	}))

	state, err = repo.Get(t.Context(), "host_cpu_high")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.False(t, state.Breaching)
	assert.Nil(t, state.Since)
	require.NotNil(t, state.LastNotifiedAt)
	assert.WithinDuration(t, notifiedAt, *state.LastNotifiedAt, time.Second)
	assert.InDelta(t, 40, state.CurrentValue, 0.001)
}

func TestAlertStatesList(t *testing.T) {
	clearAlertStates(t)
	repo := repositories.NewAlertStatesRepository(testDB)

	require.NoError(t, repo.Upsert(t.Context(), models.AlertState{
		RuleKey: "r2_usage_high", Breaching: false, Since: nil,
		LastNotifiedAt: nil, CurrentValue: 10, Threshold: 50,
	}))
	require.NoError(t, repo.Upsert(t.Context(), models.AlertState{
		RuleKey: "host_cpu_high", Breaching: true, Since: nil,
		LastNotifiedAt: nil, CurrentValue: 90, Threshold: 80,
	}))

	states, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, states, 2)
	// Alphabetical by rule_key.
	assert.Equal(t, "host_cpu_high", states[0].RuleKey)
	assert.Equal(t, "r2_usage_high", states[1].RuleKey)
}
