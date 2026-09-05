package trains_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// stationsFeed is a hand-assembled Feed exercising SearchStations directly
// against the DB path, independent of app_test.go's GTFS-zip fixtures: two
// stations sharing a name-substring plus a platform stop that must never be
// returned as a station.
func stationsFeed() *models.Feed {
	//nolint:exhaustruct //only Stops matter for station search
	return &models.Feed{
		Stops: []models.Stop{
			{StopID: "SA", Name: "Alpha", LocationType: 1},
			{StopID: "A1", Name: "Alpha", ParentStation: "SA", PlatformCode: "1"},
			{StopID: "SB", Name: "Bravo", LocationType: 1},
			{StopID: "SC", Name: "Charlie", LocationType: 1},
		},
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
			names = append(names, s.Name)
		}
		assert.Contains(t, names, "Alpha")
		assert.Contains(t, names, "Bravo")
		assert.Contains(t, names, "Charlie")
		assert.NotContains(t, names, "")
		for _, s := range stations {
			assert.NotEqual(t, "A1", s.StopID)
		}
	})

	t.Run("case-insensitive substring match", func(t *testing.T) {
		stations, err := testApp.Services.Stations.SearchStations(ctx, "rav")
		require.NoError(t, err)
		require.Len(t, stations, 1)
		assert.Equal(t, "SB", stations[0].StopID)
		assert.Equal(t, "Bravo", stations[0].Name)
	})

	t.Run("no match returns empty, not an error", func(t *testing.T) {
		stations, err := testApp.Services.Stations.SearchStations(ctx, "nowhere-at-all-xyz")
		require.NoError(t, err)
		assert.Empty(t, stations)
	})
}

func TestFeedInfoService_FeedVersion(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Services.StaticImport.Import(ctx))

	version, err := testApp.Services.FeedInfo.FeedVersion(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, version)
}
