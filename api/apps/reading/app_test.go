package reading_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/reading/internal/mocks"
	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

var testApp *reading.Reading //nolint:gochecknoglobals //needed for tests

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

// mockWebFetch / mockArxiv are testApp's external-content clients; ingest and
// feed tests register canned responses on them.
//
//nolint:gochecknoglobals //needed for tests
var mockWebFetch *mocks.MockWebFetchClient

//nolint:gochecknoglobals //needed for tests
var mockArxiv *mocks.MockArxivClient

func TestMain(m *testing.M) {
	var err error

	testCfg = testhelper.NewTestConfig()

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	fakeStore = objectstore.NewFake()
	mockWebFetch = mocks.NewMockWebFetchClient()
	mockArxiv = mocks.NewMockArxivClient()
	clients := reading.Clients{
		UniCat:      nil,
		Hardcover:   mocks.NewMockHardcoverClient(),
		ObjectStore: fakeStore,
		WebFetch:    mockWebFetch,
		Arxiv:       mockArxiv,
		// Calibre is not available in tests; produce a real (minimal) EPUB so
		// downstream KEPUB conversion still works on the result.
		HTMLConvert: func(
			_ context.Context, _, outPath string, meta services.ArticleMeta,
		) error {
			author := ""
			if len(meta.Authors) > 0 {
				author = meta.Authors[0]
			}
			return os.WriteFile(
				outPath, buildEPUBBytes(meta.Title, author, ""), 0o600,
			)
		},
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	}

	testApp = reading.NewInner(
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
	clients := reading.Clients{
		UniCat:           nil,
		Hardcover:        mocks.NewMockHardcoverClient(),
		ObjectStore:      objectstore.NewFake(),
		WebFetch:         nil,
		Arxiv:            nil,
		HTMLConvert:      nil,
		KoboStoreBaseURL: upstreamURL,
		PublicAPIBaseURL: "",
	}
	app := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return testhelper.BuildMux(app)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Reading", testApp.GetDisplayName())
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
