package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webPubGet(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.AddCookie(&accessToken)
	w := httptest.NewRecorder()
	getRoutes().ServeHTTP(w, req)
	return w
}

func TestWebPubRoutes_ManifestAndResource(t *testing.T) {
	book := addUniqueBook(t)
	seedEPUBFile(t, fakeStore, book.ID)
	base := "/books/api/book/" + book.ID.String() + "/webpub/"

	w := webPubGet(t, base+"manifest.json")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/webpub+json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"readingOrder"`)
	assert.Contains(t, w.Body.String(), `"title":"Seed Book"`)

	w = webPubGet(t, base+"positions.json")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t,
		"application/vnd.readium.position-list+json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"positions":[`)

	w = webPubGet(t, base+"res/META-INF/container.xml")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "rootfile")
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
}

// TestWebPubRoutes_ReusesOpenedEPUB: a reading session downloads the EPUB
// once; later requests read the already-opened archive.
func TestWebPubRoutes_ReusesOpenedEPUB(t *testing.T) {
	book := addUniqueBook(t)
	file := seedEPUBFile(t, fakeStore, book.ID)
	base := "/books/api/book/" + book.ID.String() + "/webpub/"

	require.Equal(t, http.StatusOK, webPubGet(t, base+"manifest.json").Code)
	require.NoError(t, fakeStore.Delete(context.Background(), file.StorageKey))

	assert.Equal(t, http.StatusOK, webPubGet(t, base+"positions.json").Code)
	w := webPubGet(t, base+"res/META-INF/container.xml")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "rootfile")
}

func TestWebPubRoutes_NotFound(t *testing.T) {
	book := addUniqueBook(t)
	seedEPUBFile(t, fakeStore, book.ID)
	base := "/books/api/book/" + book.ID.String() + "/webpub/"

	assert.Equal(t, http.StatusNotFound,
		webPubGet(t, base+"res/missing.xhtml").Code)
	assert.Equal(t, http.StatusNotFound,
		webPubGet(t, "/books/api/book/"+uuid.NewString()+"/webpub/manifest.json").Code)
	assert.Equal(t, http.StatusNotFound,
		webPubGet(t, "/books/api/book/"+uuid.NewString()+"/webpub/positions.json").Code)
	assert.Equal(t, http.StatusNotFound,
		webPubGet(t, "/books/api/book/"+uuid.NewString()+"/webpub/res/a.xhtml").Code)
	assert.Equal(t, http.StatusBadRequest,
		webPubGet(t, "/books/api/book/nope/webpub/manifest.json").Code)
}

func TestWebPubRoutes_MissingBlobIsAnError(t *testing.T) {
	book := addUniqueBook(t)
	file := seedEPUBFile(t, fakeStore, book.ID)
	require.NoError(t, fakeStore.Delete(context.Background(), file.StorageKey))

	assert.Equal(t, http.StatusInternalServerError, webPubGet(t,
		"/books/api/book/"+book.ID.String()+"/webpub/manifest.json").Code)
}
