package backlog_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// buildCoverApp creates a test Backlog with a configurable OpenLibrary client
// and a fresh fakeStore so cover cache tests are isolated.
func buildCoverApp(
	t *testing.T,
	ol openlibrary.Client,
) (*backlog.Backlog, *objectstore.FakeClient) {
	t.Helper()
	store := objectstore.NewFake()
	clients := backlog.Clients{
		SteamFactory: func(_ string) steam.Client {
			return mocks.NewMockSteamClient()
		},
		OpenLibrary:      ol,
		ObjectStore:      store,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "http://api.test",
	}
	app := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return app, store
}

// TestGetBookCover_CacheHit verifies that a cover already in R2 returns a
// presigned URL without touching the OpenLibrary client.
func TestGetBookCover_CacheHit(t *testing.T) {
	ub := addTestBook(t, "CoverCacheHitBook")
	app, store := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	// Pre-populate the cover in the fake store.
	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	require.NoError(t, store.Put(
		context.Background(),
		coverKey,
		bytes.NewReader([]byte("img")),
		3,
		"image/jpeg",
	))

	result, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Contains(t, result.URL, coverKey)
}

// TestGetBookCover_Miss_FetchesAndCaches verifies the cache-miss path:
// the cover is fetched from Open Library and stored in R2.
func TestGetBookCover_Miss_FetchesAndCaches(t *testing.T) {
	ub := addTestBook(t, "CoverMissFetchBook")
	app, store := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	result, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.NotEmpty(t, result.URL)

	// Cover should now be cached in R2.
	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	_, cached := store.GetContent(coverKey)
	assert.True(t, cached, "cover should be cached in R2 after fetch")
}

// TestGetBookCover_Miss_OpenLibraryNotFound verifies that when Open Library
// returns ErrCoverNotFound, a .missing marker is written and ErrCoverNotFound
// is returned.
func TestGetBookCover_Miss_OpenLibraryNotFound(t *testing.T) {
	ub := addTestBook(t, "CoverMissingFromOLBook")
	app, store := buildCoverApp(t, mocks.NewMockEmptyOpenLibraryClient())

	_, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)

	// A .missing marker should be written.
	missingKey := "books/" + ub.BookID.String() + "/cover.missing"
	_, hasMissing := store.GetContent(missingKey)
	assert.True(t, hasMissing, "cover.missing marker should be written in R2")
}

// TestGetBookCover_NegativeCacheHit verifies that a pre-existing .missing marker
// returns ErrCoverNotFound without calling Open Library.
func TestGetBookCover_NegativeCacheHit(t *testing.T) {
	ub := addTestBook(t, "CoverNegCacheBook")
	app, store := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	// Pre-populate the missing marker.
	missingKey := "books/" + ub.BookID.String() + "/cover.missing"
	require.NoError(t, store.Put(
		context.Background(),
		missingKey,
		bytes.NewReader([]byte{}),
		0,
		"application/octet-stream",
	))

	_, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestGetBookCover_UnknownBook verifies that a non-existent book ID returns
// ErrCoverNotFound.
func TestGetBookCover_UnknownBook(t *testing.T) {
	app, _ := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	nonExistentID := uuid.New()
	_, err := app.Services.Books.GetBookCover(context.Background(), nonExistentID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestCoverHandler_Hit verifies the cover HTTP handler issues a 302 on a hit.
func TestCoverHandler_Hit(t *testing.T) {
	ub := addTestBook(t, "CoverHandlerHitBook")
	app, store := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	// Pre-populate the cover.
	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	require.NoError(t, store.Put(
		context.Background(),
		coverKey,
		bytes.NewReader([]byte("img")),
		3,
		"image/jpeg",
	))

	mux := testhelper.BuildMux(app)
	req := httptest.NewRequest(
		http.MethodGet,
		"/backlog/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	assert.NotEmpty(t, loc)
	assert.Contains(t, w.Header().Get("Cache-Control"), "public")
}

// TestCoverHandler_NotFound verifies the cover HTTP handler returns 404 when
// no cover is available.
func TestCoverHandler_NotFound(t *testing.T) {
	app, store := buildCoverApp(t, mocks.NewMockEmptyOpenLibraryClient())

	// Use a valid book with no cover.
	ub := addTestBook(t, "CoverHandlerMissingBook")

	// Seed the missing marker directly so we skip the OL fetch.
	missingKey := "books/" + ub.BookID.String() + "/cover.missing"
	require.NoError(t, store.Put(
		context.Background(),
		missingKey,
		bytes.NewReader([]byte{}),
		0,
		"application/octet-stream",
	))

	mux := testhelper.BuildMux(app)
	req := httptest.NewRequest(
		http.MethodGet,
		"/backlog/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetBookCover_BookHasNoCoverURL verifies that when a book exists in the DB
// but has no stored cover URL, GetBookCover writes a .missing marker and returns
// ErrCoverNotFound without calling Open Library.
func TestGetBookCover_BookHasNoCoverURL(t *testing.T) {
	// addUniqueBook inserts a book with no CoverURL set.
	book := addUniqueBook(t)
	app, store := buildCoverApp(t, mocks.NewMockOpenLibraryClient())

	_, err := app.Services.Books.GetBookCover(context.Background(), book.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)

	// A .missing marker should be written so we don't re-fetch on every request.
	missingKey := "books/" + book.ID.String() + "/cover.missing"
	_, hasMissing := store.GetContent(missingKey)
	assert.True(
		t,
		hasMissing,
		"cover.missing marker should be written when book has no cover URL",
	)
}

// errFetchClient is an openlibrary.Client stub whose FetchCover always returns
// a non-404 error, exercising the "fetch failed for unknown reason" branch in
// GetBookCover.
type errFetchClient struct{}

func (errFetchClient) Search(
	_ context.Context,
	_ string,
) ([]openlibrary.ExternalBook, error) {
	return nil, errors.New("errFetchClient: Search not implemented")
}

func (errFetchClient) GetByISBN(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	return nil, errors.New("errFetchClient: GetByISBN not implemented")
}

func (errFetchClient) FetchCover(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	return nil, "", errors.New("ol: network timeout")
}

// TestGetBookCover_Miss_FetchError verifies that when Open Library returns a
// non-404 error, GetBookCover propagates that error (no .missing marker is
// written).
func TestGetBookCover_Miss_FetchError(t *testing.T) {
	ub := addTestBook(t, "CoverFetchErrorBook")
	app, store := buildCoverApp(t, errFetchClient{})

	_, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.Error(t, err)
	// The error must NOT be ErrCoverNotFound — it should propagate the underlying
	// network error.
	assert.NotErrorIs(t, err, services.ErrCoverNotFound)

	// No .missing marker should be written for transient errors.
	missingKey := "books/" + ub.BookID.String() + "/cover.missing"
	_, hasMissing := store.GetContent(missingKey)
	assert.False(
		t,
		hasMissing,
		"cover.missing must not be written for transient fetch errors",
	)
}

// TestCoverHandler_InvalidID verifies the cover HTTP handler returns 400 on bad input.
func TestCoverHandler_InvalidID(t *testing.T) {
	mux := getRoutes()
	req := httptest.NewRequest(http.MethodGet, "/backlog/api/cover/not-a-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
