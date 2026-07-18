package reading_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// buildCoverApp creates a test Backlog with a fresh fakeStore so cover cache
// tests are isolated. Covers are fetched eagerly at write time (AddToLibrary,
// resync apply, merge) — GetBookCover itself only ever reads R2.
func buildCoverApp(t *testing.T) (*reading.Reading, *objectstore.FakeClient) {
	t.Helper()
	store := objectstore.NewFake()
	clients := reading.Clients{
		UniCat:           nil,
		WebFetch:         nil,
		Arxiv:            nil,
		HTMLConvert:      nil,
		Hardcover:        nil,
		ObjectStore:      store,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "http://api.test",
	}
	app := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return app, store
}

// TestGetBookCover_CacheHit verifies that a cover already in R2 returns a
// presigned URL.
func TestGetBookCover_CacheHit(t *testing.T) {
	ub := addTestBook(t, "CoverCacheHitBook")
	app, store := buildCoverApp(t)

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

// TestGetBookCover_NotCached verifies that a book with no cached R2 cover
// returns ErrCoverNotFound — GetBookCover never live-fetches.
func TestGetBookCover_NotCached(t *testing.T) {
	ub := addTestBook(t, "CoverNotCachedBook")
	app, _ := buildCoverApp(t)

	_, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestGetBookCover_UnknownBook verifies that a non-existent book ID returns
// ErrCoverNotFound.
func TestGetBookCover_UnknownBook(t *testing.T) {
	app, _ := buildCoverApp(t)

	nonExistentID := uuid.New()
	_, err := app.Services.Books.GetBookCover(context.Background(), nonExistentID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestCoverHandler_Hit verifies the cover HTTP handler issues a 302 on a hit.
func TestCoverHandler_Hit(t *testing.T) {
	ub := addTestBook(t, "CoverHandlerHitBook")
	app, store := buildCoverApp(t)

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
		"/reading/api/cover/"+ub.BookID.String(),
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
// no cover is cached.
func TestCoverHandler_NotFound(t *testing.T) {
	app, _ := buildCoverApp(t)
	ub := addTestBook(t, "CoverHandlerMissingBook")

	mux := testhelper.BuildMux(app)
	req := httptest.NewRequest(
		http.MethodGet,
		"/reading/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCoverHandler_InvalidID verifies the cover HTTP handler returns 400 on bad input.
func TestCoverHandler_InvalidID(t *testing.T) {
	mux := getRoutes()
	req := httptest.NewRequest(http.MethodGet, "/reading/api/cover/not-a-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAddToLibrary_CachesCoverEagerly verifies that adding a book with a
// cover URL fetches the image into R2 immediately, before any cover request.
func TestAddToLibrary_CachesCoverEagerly(t *testing.T) {
	imgServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("eager-cover-bytes"))
		},
	))
	defer imgServer.Close()

	app, store := buildCoverApp(t)
	ub, err := app.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused
			Source:   "manual",
			Title:    "EagerCoverBook",
			Authors:  []string{"Test Author"},
			CoverURL: imgServer.URL,
		},
		"to-read",
		[]string{},
	)
	require.NoError(t, err)

	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	data, cached := store.GetContent(coverKey)
	require.True(t, cached, "cover should be cached in R2 right after add")
	assert.Equal(t, "eager-cover-bytes", string(data))
}

// TestAddToLibrary_CoverFetchFailure_DoesNotBlockAdd verifies that a failing
// cover fetch never blocks the add itself.
func TestAddToLibrary_CoverFetchFailure_DoesNotBlockAdd(t *testing.T) {
	app, store := buildCoverApp(t)
	ub, err := app.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused
			Source:   "manual",
			Title:    "BadCoverURLBook",
			Authors:  []string{"Test Author"},
			CoverURL: "http://127.0.0.1:1/unreachable.jpg",
		},
		"to-read",
		[]string{},
	)
	require.NoError(t, err)

	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	_, cached := store.GetContent(coverKey)
	assert.False(t, cached, "no cover should be cached when the fetch fails")
}
