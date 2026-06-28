package backlog_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

var testApp *backlog.Backlog //nolint:gochecknoglobals //needed for tests

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

func TestMain(m *testing.M) {
	var err error

	testCfg = config.New(logging.NewNopLogger())
	testCfg.Env = configtools.TestEnv
	testCfg.Throttle = false
	testCfg.SteamAPIKey = "test-steam-api-key"

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	fakeStore = objectstore.NewFake()
	clients := backlog.Clients{
		SteamFactory: func(_ string) steam.Client {
			return mocks.NewMockSteamClient()
		},
		OpenLibrary:      mocks.NewMockOpenLibraryClient(),
		GoogleBooks:      nil,
		ObjectStore:      fakeStore,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	}

	testApp = backlog.NewInner(
		context.Background(),
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

	err = testApp.SaveIntegrations(
		context.Background(),
		userID,
		//nolint:exhaustruct //intentionally empty to mark user as onboarded
		backlog.Integrations{},
	)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

// getRoutesWithKoboUpstream creates a Backlog instance identical to testApp
// but with a custom KoboStoreBaseURL (for proxy/merge tests).
// It shares the same DB so tokens generated via testApp are recognised.
func getRoutesWithKoboUpstream(t *testing.T, upstreamURL string) http.Handler {
	t.Helper()
	clients := backlog.Clients{
		SteamFactory: func(_ string) steam.Client {
			return mocks.NewMockSteamClient()
		},
		OpenLibrary:      mocks.NewMockOpenLibraryClient(),
		GoogleBooks:      nil,
		ObjectStore:      objectstore.NewFake(),
		KoboStoreBaseURL: upstreamURL,
		PublicAPIBaseURL: "",
	}
	app := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return testhelper.BuildMux(app)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Backlog", testApp.GetDisplayName())
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
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //ISBN intentionally absent
		Provider:   "manual",
		ProviderID: fmt.Sprintf("noisbn-%s", title),
		Title:      title,
		Authors:    []string{"Test Author"},
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
	isbn := "9780140449112"
	cover := "https://example.com/cover.jpg"
	desc := "Test description."
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //optional ISBN10 not needed
		Provider:    "manual",
		ProviderID:  fmt.Sprintf("test-%s", title),
		Title:       title,
		Authors:     []string{"Test Author"},
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
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
