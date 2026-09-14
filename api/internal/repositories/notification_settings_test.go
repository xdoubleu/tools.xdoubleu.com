package repositories_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
)

func TestNotificationSettingsListReturnsSeededSources(t *testing.T) {
	repo := repositories.NewNotificationSettingsRepository(testDB)

	settings, err := repo.List(t.Context())
	require.NoError(t, err)

	got := make(map[repositories.NotificationSource]bool, len(settings))
	for _, s := range settings {
		got[s.SourceKey] = s.Enabled
	}
	require.Contains(t, got, repositories.NotificationSourceUnhealthyFeeds)
	require.Contains(t, got, repositories.NotificationSourceOpenFeedItems)
}

func TestNotificationSettingsIsEnabledDefaultsTrue(t *testing.T) {
	repo := repositories.NewNotificationSettingsRepository(testDB)

	enabled, err := repo.IsEnabled(
		t.Context(),
		repositories.NotificationSourceUnhealthyFeeds,
	)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestNotificationSettingsIsEnabledDefaultsTrueForUnknownSource(t *testing.T) {
	repo := repositories.NewNotificationSettingsRepository(testDB)

	enabled, err := repo.IsEnabled(
		t.Context(),
		repositories.NotificationSource("unknown"),
	)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestNotificationSettingsSetEnabledRoundTrips(t *testing.T) {
	repo := repositories.NewNotificationSettingsRepository(testDB)
	t.Cleanup(func() {
		require.NoError(t, repo.SetEnabled(
			context.Background(), repositories.NotificationSourceUnhealthyFeeds, true,
		))
	})

	require.NoError(t, repo.SetEnabled(
		t.Context(), repositories.NotificationSourceUnhealthyFeeds, false,
	))

	enabled, err := repo.IsEnabled(
		t.Context(),
		repositories.NotificationSourceUnhealthyFeeds,
	)
	require.NoError(t, err)
	require.False(t, enabled)
}
