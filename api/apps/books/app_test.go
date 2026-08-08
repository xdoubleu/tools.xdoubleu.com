package books_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

var testApp *books.Books //nolint:gochecknoglobals //needed for tests

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

// fakeStore is the shared in-memory object store used by testApp.
// Tests can Put bytes directly then call FinalizeUpload to simulate R2 uploads.
var fakeStore *objectstore.FakeClient //nolint:gochecknoglobals //needed for tests

// mockWebFetch is testApp's external-content client; ingest and feed tests
// register canned responses on it.
//
//nolint:gochecknoglobals //needed for tests
var mockWebFetch *mocks.MockWebFetchClient

func TestMain(m *testing.M) {
	var err error

	testCfg = testhelper.NewTestConfig()

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	fakeStore = objectstore.NewFake()
	mockWebFetch = mocks.NewMockWebFetchClient()
	clients := books.Clients{
		UniCat:           nil,
		Hardcover:        mocks.NewMockHardcoverClient(),
		ObjectStore:      fakeStore,
		WebFetch:         mockWebFetch,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	}

	testApp = books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		clients,
	)

	err = testApp.ApplyMigrations(context.Background(), postgresDB)
	if err != nil {
		panic(err)
	}

	ensureGlobalJobRuns(postgresDB)

	os.Exit(m.Run())
}

// ensureGlobalJobRuns mirrors cmd/api/migrations/00005_observability.sql's
// job_runs table so this package's Start()/jobqueue.AddJob calls can look up
// a job's last successful run before the cmd/api package has applied the
// global migrations.
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

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

// getRoutesWithKoboUpstream creates a Backlog instance identical to testApp
// but with a custom KoboStoreBaseURL (for proxy/merge tests).
// It shares the same DB so tokens generated via testApp are recognised.
func getRoutesWithKoboUpstream(t *testing.T, upstreamURL string) http.Handler {
	t.Helper()
	clients := books.Clients{
		UniCat:           nil,
		Hardcover:        mocks.NewMockHardcoverClient(),
		ObjectStore:      objectstore.NewFake(),
		WebFetch:         nil,
		KoboStoreBaseURL: upstreamURL,
		PublicAPIBaseURL: "",
	}
	app := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return testhelper.BuildMux(app)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Books", testApp.GetDisplayName())
}

// goodreadsCSVForImport is a minimal Goodreads CSV for import testing.
//
//nolint:lll // CSV rows are inherently long
const goodreadsCSVForImport = `Book Id,Title,Author,ISBN,ISBN13,My Rating,Exclusive Shelf,Bookshelves with positions,Date Read
99001,Import Test Book,Import Author,"=""0140449116""","=""9780140449112""",4,read,"read (#1)",2023/05/20
`

// addTestBookNoISBN adds a book without an ISBN so each call creates a distinct
// catalog entry (ISBN is the dedup key; without it each ProviderID gets its own row).
func addTestBookNoISBN(t *testing.T, title string) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //ISBN intentionally absent
		Source:  "manual",
		Title:   title,
		Authors: []string{"Test Author"},
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		ext,
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

func addTestBook(t *testing.T, title string) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused
		Source:      "manual",
		Title:       title,
		Authors:     []string{"Test Author"},
		ISBN13:      "9780140449112",
		CoverURL:    "https://example.com/cover.jpg",
		Description: "Test description.",
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		ext,
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}
