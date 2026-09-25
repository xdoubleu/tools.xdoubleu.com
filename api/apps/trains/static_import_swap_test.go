package trains_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// swapFeed builds a single-stop Feed to prove a second import replaces the
// first.
func swapFeed(stopID, feedVersion string) *models.Feed {
	//nolint:exhaustruct //only Stops/Info matter for this test
	return &models.Feed{
		Stops: []models.Stop{
			{
				StopID: stopID, NameNL: "Zulu", NameFR: "Zulu", NameEN: "Zulu",
				LocationType: 1,
			},
		},
		Info: models.FeedInfo{FeedVersion: feedVersion}, //nolint:exhaustruct //test fixture
	}
}

// TestFeedRepository_ImportFeed_SecondImportFullyReplacesFirst: live tables
// hold only the second feed, with feed_info swapped in the same transaction.
func TestFeedRepository_ImportFeed_SecondImportFullyReplacesFirst(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	require.NoError(
		t, testApp.Repositories.Feed.ImportFeed(ctx, swapFeed("ZX1", "2026-09-01")),
	)
	require.NoError(
		t, testApp.Repositories.Feed.ImportFeed(ctx, swapFeed("ZX2", "2026-09-02")),
	)

	stops, err := testApp.Repositories.Feed.AllStops(ctx)
	require.NoError(t, err)
	ids := make([]string, 0, len(stops))
	for _, s := range stops {
		ids = append(ids, s.StopID)
	}
	assert.ElementsMatch(
		t, []string{"ZX2"}, ids,
		"second import must fully replace the first, not merge with it",
	)

	info, err := testApp.Repositories.Feed.GetFeedInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "2026-09-02", info.FeedVersion)
}

// TestFeedRepository_PopulateStaging_IndependentOfLiveTableLocks:
// PopulateStaging touches only `_staging`, so it completes while every live
// table is exclusively locked.
func TestFeedRepository_PopulateStaging_IndependentOfLiveTableLocks(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	lockTx, err := testDB.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = lockTx.Rollback(ctx) })
	_, err = lockTx.Exec(ctx, `
		LOCK TABLE trains.stops, trains.routes, trains.trips,
		           trains.stop_times, trains.calendar_dates, trains.transfers
		IN ACCESS EXCLUSIVE MODE
	`)
	require.NoError(
		t, err, "test setup: acquiring the live-table lock must itself succeed",
	)

	popCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(
		t,
		testApp.Repositories.Feed.PopulateStaging(popCtx, swapFeed("ZX3", "2026-09-03")),
		"PopulateStaging must not wait on a lock held against the live tables",
	)
}

// TestFeedRepository_AllStops_NotBlockedByStagingTableLock: live reads
// complete while `_staging` is locked by an in-flight import.
func TestFeedRepository_AllStops_NotBlockedByStagingTableLock(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	lockTx, err := testDB.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = lockTx.Rollback(ctx) })
	_, err = lockTx.Exec(ctx, `
		LOCK TABLE trains.stops_staging, trains.routes_staging,
		           trains.trips_staging, trains.stop_times_staging,
		           trains.calendar_dates_staging, trains.transfers_staging
		IN ACCESS EXCLUSIVE MODE
	`)
	require.NoError(
		t, err, "test setup: acquiring the staging-table lock must itself succeed",
	)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stops, err := testApp.Repositories.Feed.AllStops(readCtx)
	require.NoError(t, err, "a live-table read must not wait on the staging-table lock")
	assert.NotEmpty(t, stops)
}

// TestFeedRepository_ImportFeed_PropagatesPopulateStagingError: a PK
// violation in staging fails ImportFeed before SwapStagingIn runs.
func TestFeedRepository_ImportFeed_PropagatesPopulateStagingError(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	//nolint:exhaustruct //only Stops matters for this test
	badFeed := &models.Feed{
		Stops: []models.Stop{
			{StopID: "DUP", NameNL: "X", NameFR: "X", NameEN: "X", LocationType: 1},
			{StopID: "DUP", NameNL: "Y", NameFR: "Y", NameEN: "Y", LocationType: 1},
		},
	}
	err := testApp.Repositories.Feed.ImportFeed(ctx, badFeed)
	require.Error(t, err)

	// Live tables are untouched by the failed import.
	stops, allErr := testApp.Repositories.Feed.AllStops(ctx)
	require.NoError(t, allErr)
	ids := make([]string, 0, len(stops))
	for _, s := range stops {
		ids = append(ids, s.StopID)
	}
	assert.NotContains(t, ids, "DUP")
	assert.Contains(t, ids, "SA")
}

// TestFeedRepository_PopulateStaging_TruncateFailureIsReturned: TRUNCATE
// fails when stops_staging is renamed away.
func TestFeedRepository_PopulateStaging_TruncateFailureIsReturned(t *testing.T) {
	ctx := context.Background()
	_, err := testDB.Exec(
		ctx, "ALTER TABLE trains.stops_staging RENAME TO stops_staging_missing_test",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, restoreErr := testDB.Exec(
			ctx, "ALTER TABLE trains.stops_staging_missing_test RENAME TO stops_staging",
		)
		require.NoError(t, restoreErr)
	})

	popErr := testApp.Repositories.Feed.PopulateStaging(ctx, swapFeed("ZX4", "2026-09-04"))
	require.Error(t, popErr)
}

// TestFeedRepository_SwapStagingIn_RenameFailureIsReturned: live->tmp fails
// when stops_swap_tmp already exists.
func TestFeedRepository_SwapStagingIn_RenameFailureIsReturned(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	_, err := testDB.Exec(ctx, "CREATE TABLE trains.stops_swap_tmp (id TEXT)")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := testDB.Exec(ctx, "DROP TABLE IF EXISTS trains.stops_swap_tmp")
		require.NoError(t, dropErr)
	})

	swapErr := testApp.Repositories.Feed.SwapStagingIn(
		ctx, models.FeedInfo{FeedVersion: "2026-09-05"}, //nolint:exhaustruct //test fixture
	)
	require.Error(t, swapErr)
}

// TestFeedRepository_SwapStagingIn_SecondRenameFailureIsReturned: a failure
// mid-loop (staging->live) rolls back the whole swap.
func TestFeedRepository_SwapStagingIn_SecondRenameFailureIsReturned(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	_, err := testDB.Exec(
		ctx, "ALTER TABLE trains.routes_staging RENAME TO routes_staging_missing_test",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, restoreErr := testDB.Exec(
			ctx,
			"ALTER TABLE trains.routes_staging_missing_test RENAME TO routes_staging",
		)
		require.NoError(t, restoreErr)
	})

	swapErr := testApp.Repositories.Feed.SwapStagingIn(
		ctx, models.FeedInfo{FeedVersion: "2026-09-06"}, //nolint:exhaustruct //test fixture
	)
	require.Error(t, swapErr)

	// Tables renamed before the failure are rolled back.
	stops, allErr := testApp.Repositories.Feed.AllStops(ctx)
	require.NoError(t, allErr)
	assert.NotEmpty(t, stops)
}

// The duplicate*Feed helpers each carry one primary-key duplicate.

func duplicateRouteFeed() *models.Feed {
	//nolint:exhaustruct //only Routes matters for this case
	return &models.Feed{
		Routes: []models.Route{
			{RouteID: "DUPR", RouteType: 2},
			{RouteID: "DUPR", RouteType: 2},
		},
	}
}

func duplicateTripFeed() *models.Feed {
	//nolint:exhaustruct //only Trips matters for this case
	return &models.Feed{
		Trips: []models.Trip{
			{TripID: "DUPT", RouteID: "r1", ServiceID: "s1"},
			{TripID: "DUPT", RouteID: "r1", ServiceID: "s1"},
		},
	}
}

func duplicateStopTimeFeed() *models.Feed {
	//nolint:exhaustruct //only StopTimes matters for this case
	return &models.Feed{
		StopTimes: []models.StopTime{
			{TripID: "t1", StopSequence: 1, StopID: "s1"},
			{TripID: "t1", StopSequence: 1, StopID: "s2"},
		},
	}
}

func duplicateCalendarDateFeed() *models.Feed {
	day := trainsDayStart()
	//nolint:exhaustruct //only CalendarDates matters for this case
	return &models.Feed{
		CalendarDates: []models.CalendarDate{
			{ServiceID: "s1", Date: day, ExceptionType: 1},
			{ServiceID: "s1", Date: day, ExceptionType: 1},
		},
	}
}

func duplicateTransferFeed() *models.Feed {
	//nolint:exhaustruct //only Transfers matters for this case
	return &models.Feed{
		Transfers: []models.Transfer{
			{FromStopID: "s1", ToStopID: "s2"},
			{FromStopID: "s1", ToStopID: "s2"},
		},
	}
}

// TestFeedRepository_PopulateStaging_EachCopyFailureBranchIsReturned covers
// the remaining COPY failure branches.
func TestFeedRepository_PopulateStaging_EachCopyFailureBranchIsReturned(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	t.Cleanup(func() {
		require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	})

	cases := map[string]*models.Feed{
		"duplicate route_id":                   duplicateRouteFeed(),
		"duplicate trip_id":                    duplicateTripFeed(),
		"duplicate (trip_id, stop_sequence)":   duplicateStopTimeFeed(),
		"duplicate (service_id, date)":         duplicateCalendarDateFeed(),
		"duplicate (from_stop_id, to_stop_id)": duplicateTransferFeed(),
	}

	for name, feed := range cases {
		t.Run(name, func(t *testing.T) {
			err := testApp.Repositories.Feed.PopulateStaging(ctx, feed)
			require.Error(t, err)
		})
	}
}
