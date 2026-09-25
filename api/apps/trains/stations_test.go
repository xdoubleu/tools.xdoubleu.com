package trains_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// stationsFeed is a hand-built Feed for SearchStations and GetFeedInfo: two
// stations sharing a name substring, a platform that is never a station, and
// FeedInfo with translation coverage. Import it via ImportFeed; the shared
// mock serves a real body only once per test binary.
func stationsFeed() *models.Feed {
	//nolint:exhaustruct //only Stops/Info matter for station and feed-info search
	return &models.Feed{
		Stops: []models.Stop{
			{
				StopID:       "SA",
				NameNL:       "Alpha",
				NameFR:       "Alpha",
				NameEN:       "Alpha",
				LocationType: 1,
			},
			{
				StopID: "A1", NameNL: "Alpha", NameFR: "Alpha", NameEN: "Alpha",
				ParentStation: "SA", PlatformCode: "1",
			},
			{
				StopID: "SB", NameNL: "Bravo-NL", NameFR: "Bravo", NameEN: "Bravo-EN",
				LocationType: 1,
			},
			{
				StopID: "SC", NameNL: "Charlie", NameFR: "Charlie", NameEN: "Charlie",
				LocationType: 1,
			},
		},
		Info: models.FeedInfo{
			FeedVersion: "2026-08-31",
			Translations: models.TranslationCoverage{
				StopsNL: 2, StopsFR: 0, StopsEN: 2, Rows: 5, RowsUnmatched: 1,
			},
		}, //nolint:exhaustruct //rest unused
	}
}

func TestStationsService_SearchStations(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))

	t.Run("empty query returns every station, never a platform", func(t *testing.T) {
		stations, err := testApp.Services.Stations.SearchStations(ctx, "")
		require.NoError(t, err)
		names := make([]string, 0, len(stations))
		for _, s := range stations {
			names = append(names, s.NameFR)
		}
		assert.Contains(t, names, "Alpha")
		assert.Contains(t, names, "Bravo")
		assert.Contains(t, names, "Charlie")
		assert.NotContains(t, names, "")
		for _, s := range stations {
			assert.NotEqual(t, "A1", s.StopID)
		}
	})

	t.Run(
		"case-insensitive substring match against the French name",
		func(t *testing.T) {
			stations, err := testApp.Services.Stations.SearchStations(ctx, "rav")
			require.NoError(t, err)
			require.Len(t, stations, 1)
			assert.Equal(t, "SB", stations[0].StopID)
			assert.Equal(t, "Bravo", stations[0].NameFR)
		},
	)

	t.Run("matches a name only present in another language", func(t *testing.T) {
		stations, err := testApp.Services.Stations.SearchStations(ctx, "bravo-nl")
		require.NoError(t, err)
		require.Len(t, stations, 1)
		assert.Equal(t, "SB", stations[0].StopID)

		stations, err = testApp.Services.Stations.SearchStations(ctx, "bravo-en")
		require.NoError(t, err)
		require.Len(t, stations, 1)
		assert.Equal(t, "SB", stations[0].StopID)
	})

	t.Run("no match returns empty, not an error", func(t *testing.T) {
		stations, err := testApp.Services.Stations.SearchStations(
			ctx,
			"nowhere-at-all-xyz",
		)
		require.NoError(t, err)
		assert.Empty(t, stations)
	})
}

func TestFeedInfoService_FeedInfo(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))

	info, err := testApp.Services.FeedInfo.FeedInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-31", info.FeedVersion)
	// imported_at is set by the database on every import.
	require.NotNil(t, info.ImportedAt)
	assert.WithinDuration(t, time.Now(), *info.ImportedAt, time.Minute)
}

// TestFeedInfoService_FeedInfo_NothingImported: a fresh replica returns
// empty values, not an error.
func TestFeedInfoService_FeedInfo_NothingImported(t *testing.T) {
	ctx := context.Background()
	_, err := testDB.Exec(ctx, `TRUNCATE trains.feed_info`)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(
			t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()),
		)
	})

	info, err := testApp.Services.FeedInfo.FeedInfo(ctx)
	require.NoError(t, err)
	assert.Empty(t, info.FeedVersion)
	assert.Nil(t, info.ImportedAt)
}
