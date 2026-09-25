package trains_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains"
	"tools.xdoubleu.com/apps/trains/internal/jobs"
	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //shared across tests
var (
	testApp *trains.Trains
	testDB  postgres.DB
	testBMC *mocks.MockBMCClient
	userID  = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"
)

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"

	pool := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = pool

	testBMC = mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles()))
	testApp = trains.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		pool,
		testBMC,
	)

	if err := testApp.ApplyMigrations(context.Background(), pool); err != nil {
		panic(err)
	}

	ensureGlobalJobRuns(pool)

	os.Exit(m.Run())
}

// trainsDayStart is today's midnight in Europe/Brussels, the service date
// journey detail and realtime correlation key off. Anchor test feeds here,
// not UTC midnight: the two diverge late in the UTC day.
func trainsDayStart() time.Time {
	loc, err := time.LoadLocation("Europe/Brussels")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
}

// ensureGlobalJobRuns creates cmd/api's job_runs table, which Start needs
// before the global migrations have run in this package's tests.
func ensureGlobalJobRuns(db postgres.DB) {
	ctx := context.Background()
	if _, err := db.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS global"); err != nil {
		panic(err)
	}

	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS global.job_runs (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			job_id TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			duration_ms BIGINT NOT NULL,
			success BOOLEAN NOT NULL,
			error TEXT
		)
	`)
	if err != nil {
		panic(err)
	}
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "trains", testApp.GetName())
	assert.Equal(t, "Trains", testApp.GetDisplayName())
}

// TestStaticImport_ResolvesTripsFromCalendarDates: with an all-zero decoy
// calendar.txt, trips must still resolve from calendar_dates alone.
func TestStaticImport_ResolvesTripsFromCalendarDates(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Services.StaticImport.Import(ctx))

	date, err := time.Parse("2006-01-02", mocks.SampleServiceDate)
	require.NoError(t, err)

	count, err := testApp.Repositories.Feed.CountTripsResolvingOn(ctx, date)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both trip_522 variants run on the service date")

	info, err := testApp.Repositories.Feed.GetFeedInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "2026-08-31", info.FeedVersion)
	assert.Equal(t, `"v1"`, info.ETag)
}

// TestStaticImport_UnchangedFeedIsNoOp checks the second run sends stored
// validators and short-circuits on 304.
func TestStaticImport_UnchangedFeedIsNoOp(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Services.StaticImport.Import(ctx))
	before := len(testBMC.Calls)

	require.NoError(t, testApp.Services.StaticImport.Import(ctx))

	require.Greater(t, len(testBMC.Calls), before)
	last := testBMC.Calls[len(testBMC.Calls)-1]
	assert.Equal(t, `"v1"`, last.ETag, "conditional GET armed from stored feed_info")

	date, err := time.Parse("2006-01-02", mocks.SampleServiceDate)
	require.NoError(t, err)
	count, err := testApp.Repositories.Feed.CountTripsResolvingOn(ctx, date)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "no-op run leaves the timetable intact")
}

// TestStaticImport_RecordsPhaseDurations checks a full import records
// fetch, parse and import durations in job_phase_duration_seconds.
func TestStaticImport_RecordsPhaseDurations(t *testing.T) {
	ctx := context.Background()
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"
	bmcClient := mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles()))
	app := trains.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		testDB,
		bmcClient,
	)

	// A stale parser_version drops the validators, forcing a full import.
	_, err := testDB.Exec(ctx,
		`UPDATE trains.feed_info SET parser_version = 1 WHERE singleton`)
	require.NoError(t, err)

	require.NoError(t, app.Services.StaticImport.Import(ctx))

	for _, phase := range []string{"fetch", "parse", "import"} {
		assert.Positive(t, jobPhaseSampleCount(t, services.StaticImportJobID, phase),
			"phase %q must be recorded in job_phase_duration_seconds", phase)
	}
}

func jobPhaseSampleCount(t *testing.T, job, phase string) uint64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, mf := range families {
		if mf.GetName() != "job_phase_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["job"] == job && labels["phase"] == phase {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

// TestStaticImport_ParserVersionMismatchForcesReimport: rows from an older
// importer must be reimported even when the feed's ETag is unchanged. Uses its
// own app and mock so the client serves a real body again.
func TestStaticImport_ParserVersionMismatchForcesReimport(t *testing.T) {
	ctx := context.Background()
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"
	bmcClient := mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles()))
	app := trains.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		testDB,
		bmcClient,
	)

	require.NoError(t, app.Services.StaticImport.Import(ctx))
	info, err := app.Repositories.Feed.GetFeedInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, services.ImportParserVersion, info.ParserVersion,
		"an import stamps the current importer")
	assert.NotNil(t, info.ImportedAt)

	// Simulate an older importer: French stop_name in every language, current ETag.
	_, err = testDB.Exec(ctx,
		`UPDATE trains.feed_info SET parser_version = 1 WHERE singleton`)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx,
		`UPDATE trains.stops SET name_nl = name_fr, name_en = name_fr`)
	require.NoError(t, err)

	bmcClient.Calls = nil
	require.NoError(t, app.Services.StaticImport.Import(ctx))

	require.Len(t, bmcClient.Calls, 1)
	assert.Empty(t, bmcClient.Calls[0].ETag,
		"validators dropped, so the unchanged feed is fetched in full")
	assert.Empty(t, bmcClient.Calls[0].LastModified)

	stations, err := app.Services.Stations.SearchStations(ctx, "brussel-zuid")
	require.NoError(t, err)
	require.Len(t, stations, 1, "the forced re-import restores the Dutch name")
	assert.Equal(t, "Bruxelles-Midi", stations[0].NameFR)
	assert.Equal(t, "Brussels-South", stations[0].NameEN)

	info, err = app.Repositories.Feed.GetFeedInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, services.ImportParserVersion, info.ParserVersion)
}

// TestStaticImport_ResumesConditionalGetAfterVersionBump: after a
// version-forced reimport, the next run must resume conditional GET.
func TestStaticImport_ResumesConditionalGetAfterVersionBump(t *testing.T) {
	ctx := context.Background()
	cfg := testhelper.NewTestConfig()
	cfg.BMCPartnerKey = "test-key"
	bmcClient := mocks.NewMockBMCClient(mocks.BuildFeedZip(mocks.SampleFeedFiles()))
	app := trains.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		testDB,
		bmcClient,
	)

	require.NoError(t, app.Services.StaticImport.Import(ctx))

	// Simulate a prior importer's rows to force a second unconditional fetch.
	_, err := testDB.Exec(ctx,
		`UPDATE trains.feed_info SET parser_version = 1 WHERE singleton`)
	require.NoError(t, err)

	bmcClient.Calls = nil
	require.NoError(t, app.Services.StaticImport.Import(ctx))
	require.Len(t, bmcClient.Calls, 1)
	assert.Empty(t, bmcClient.Calls[0].ETag,
		"version mismatch: validators dropped, forced unconditional fetch")

	info, err := app.Repositories.Feed.GetFeedInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, services.ImportParserVersion, info.ParserVersion,
		"the forced reimport stamps the current importer")
	require.NotEmpty(t, info.ETag,
		"the forced reimport also stores fresh validators to resume from")

	// Third run must send the stored ETag, not fetch unconditionally.
	require.NoError(t, app.Services.StaticImport.Import(ctx))
	require.Len(t, bmcClient.Calls, 2)
	assert.Equal(t, info.ETag, bmcClient.Calls[1].ETag,
		"parser version now matches: conditional GET resumed using the "+
			"validators the forced reimport just stored")
}

func TestStaticImport_MissingKeyIsSkippedNotFailed(t *testing.T) {
	ctx := context.Background()
	testBMC.Err = bmc.ErrNotConfigured
	t.Cleanup(func() { testBMC.Err = nil })

	assert.NoError(t, testApp.Services.StaticImport.Import(ctx))
}

func TestStaticImport_FetchErrorPropagates(t *testing.T) {
	ctx := context.Background()
	testBMC.Err = errors.New("gateway down")
	t.Cleanup(func() { testBMC.Err = nil })

	assert.Error(t, testApp.Services.StaticImport.Import(ctx))
}

func TestNewAndStart(t *testing.T) {
	// no BMC_PARTNER_KEY set — exercises the warn path in New.
	cfg := testhelper.NewTestConfig()
	a := trains.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(), cfg, testDB,
	)
	assert.Empty(t, a.GetDomain())
	a.Routes("trains", http.NewServeMux())
	require.NoError(t, a.Start())
}

func TestStaticImportJob_Metadata(t *testing.T) {
	j := jobs.NewStaticImportJob(testApp.Services.StaticImport)
	assert.Equal(t, "trains-static-import", j.ID())
	assert.Equal(t, 24*time.Hour, j.RunEvery())
	assert.NoError(t, j.Run(context.Background(), logging.NewNopLogger()))
}
