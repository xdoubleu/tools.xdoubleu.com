package books_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// TestNew_ReturnsApp covers the production New constructor that wires real
// UniCat / Hardcover / R2 clients.
func TestNew_ReturnsApp(t *testing.T) {
	bl := books.New(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
	)
	require.NotNil(t, bl)
}

// TestStart_RegistersJobs covers Start registering the resync job and topics.
func TestStart_RegistersJobs(t *testing.T) {
	bl := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Hardcover:        nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NotNil(t, bl)
	err := bl.Start()
	require.NoError(t, err)
}

// TestStartWorkers_RegistersHeavyJobs: the workers role registers the jobs
// Start leaves out, so both can share one job queue's ID space.
func TestStartWorkers_RegistersHeavyJobs(t *testing.T) {
	bl := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Hardcover:        nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, bl.StartWorkers())
	require.NoError(t, bl.Start())
}

// StartStorageScan returns at once; the scan runs on the job queue and is
// recorded in job_runs.
func TestStartStorageScan_RunsInBackground(t *testing.T) {
	bl := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Hardcover:        nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, bl.Start())
	since := time.Now().Add(-time.Second)

	bl.StartStorageScan()

	jobRuns := sharedrepos.NewJobRunsRepository(testDB)
	require.Eventually(t, func() bool {
		last, err := jobRuns.LastSuccessAt(context.Background(), "books-storage-scan")
		return err == nil && last != nil && last.After(since)
	}, 10*time.Second, 50*time.Millisecond)
}
