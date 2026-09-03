package trains_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains"
	"tools.xdoubleu.com/apps/trains/internal/jobs"
	"tools.xdoubleu.com/apps/trains/internal/mocks"
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

	os.Exit(m.Run())
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "trains", testApp.GetName())
	assert.Equal(t, "Trains", testApp.GetDisplayName())
}

// TestStaticImport_ResolvesTripsFromCalendarDates is the assertion issue
// #1390 calls for: after importing a feed whose calendar.txt is an all-zero
// decoy, non-zero trips must still resolve for a future date — purely from
// calendar_dates.
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

// TestStaticImport_UnchangedFeedIsNoOp proves the second run sends the
// stored validators and short-circuits on the mock's 304.
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
