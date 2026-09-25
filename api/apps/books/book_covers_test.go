package books_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// hangingObjectStore blocks Exists until the context is cancelled.
type hangingObjectStore struct {
	objectstore.Client
}

func (h hangingObjectStore) Exists(ctx context.Context, _ string) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

// buildCoverApp creates an app with its own fake store.
func buildCoverApp(t *testing.T) (*books.Books, *objectstore.FakeClient) {
	t.Helper()
	store := objectstore.NewFake()
	clients := books.Clients{
		UniCat:           nil,
		WebFetch:         nil,
		Hardcover:        nil,
		ObjectStore:      store,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "http://api.test",
	}
	app := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)
	return app, store
}

// TestGetBookCover_CacheHit checks a cached cover returns a presigned URL.
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

// TestGetBookCover_NotCached checks ErrCoverNotFound without a cover.
func TestGetBookCover_NotCached(t *testing.T) {
	ub := addTestBook(t, "CoverNotCachedBook")
	app, _ := buildCoverApp(t)

	_, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestGetBookCover_SelfHealsOnMiss: a CoverURL without an R2 object is refetched.
func TestGetBookCover_SelfHealsOnMiss(t *testing.T) {
	imgServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("self-healed-cover-bytes"))
		},
	))
	defer imgServer.Close()

	app, store := buildCoverApp(t)
	ub, err := app.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused
			Source:   "manual",
			Title:    "SelfHealCoverBook",
			Authors:  []string{"Test Author"},
			CoverURL: imgServer.URL,
		},
		"to-read",
		[]string{},
	)
	require.NoError(t, err)

	coverKey := "books/" + ub.BookID.String() + "/cover.jpg"
	require.NoError(t, store.Delete(context.Background(), coverKey))
	_, cached := store.GetContent(coverKey)
	require.False(t, cached, "precondition: cache should be empty after delete")

	result, err := app.Services.Books.GetBookCover(context.Background(), ub.BookID)
	require.NoError(t, err)
	assert.Contains(t, result.URL, coverKey)

	data, cached := store.GetContent(coverKey)
	require.True(t, cached, "cover should be re-cached after self-heal")
	assert.Equal(t, "self-healed-cover-bytes", string(data))
}

// TestGetBookCover_UnknownBook checks ErrCoverNotFound for an unknown ID.
func TestGetBookCover_UnknownBook(t *testing.T) {
	app, _ := buildCoverApp(t)

	nonExistentID := uuid.New()
	_, err := app.Services.Books.GetBookCover(context.Background(), nonExistentID)
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrCoverNotFound)
}

// TestCoverHandler_Hit checks the 302 on a hit.
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
		"/books/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	assert.NotEmpty(t, loc)
	assert.Contains(t, w.Header().Get("Cache-Control"), "public")
}

// TestCoverHandler_NotFound checks the 404 without a cover.
func TestCoverHandler_NotFound(t *testing.T) {
	app, _ := buildCoverApp(t)
	ub := addTestBook(t, "CoverHandlerMissingBook")

	mux := testhelper.BuildMux(app)
	req := httptest.NewRequest(
		http.MethodGet,
		"/books/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCoverHandler_InvalidID checks the 400 on bad input.
func TestCoverHandler_InvalidID(t *testing.T) {
	mux := getRoutes()
	req := httptest.NewRequest(http.MethodGet, "/books/api/cover/not-a-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCoverHandler_HangingObjectStoreFailsFast: a stalled R2 call must not
// hang the handler past the write timeout.
func TestCoverHandler_HangingObjectStoreFailsFast(t *testing.T) {
	ub := addTestBook(t, "CoverHandlerHangingStoreBook")

	clients := books.Clients{
		UniCat:           nil,
		WebFetch:         nil,
		Hardcover:        nil,
		ObjectStore:      hangingObjectStore{Client: objectstore.NewFake()},
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "http://api.test",
	}
	app := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		clients,
	)

	mux := testhelper.BuildMux(app)
	req := httptest.NewRequest(
		http.MethodGet,
		"/books/api/cover/"+ub.BookID.String(),
		nil,
	)
	w := httptest.NewRecorder()

	start := time.Now()
	mux.ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Less(t, elapsed, 9*time.Second, "must fail fast, not hang")
}

// TestAddToLibrary_CachesCoverEagerly checks the cover is fetched on add.
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

// TestAddToLibrary_CoverFetchFailure_DoesNotBlockAdd checks the add succeeds.
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
