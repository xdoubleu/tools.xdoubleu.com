package books_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// TestKoboLibrarySync_ConcurrentSyncsWarmOnce: a sync while the user's cover
// warm is still running doesn't start another.
func TestKoboLibrarySync_ConcurrentSyncsWarmOnce(t *testing.T) {
	ctx := context.Background()
	rawToken, bookID := setupKoboSyncBook(t, "kobo-warm-once-"+uuid.NewString())

	var hits atomic.Int32
	release := make(chan struct{})
	cover := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			select {
			case <-release:
			case <-r.Context().Done():
			}
			_, _ = w.Write([]byte(jpegCover))
		},
	))
	t.Cleanup(cover.Close)
	t.Cleanup(func() { close(release) })
	_, err := testDB.Exec(ctx,
		`UPDATE books.books SET cover_url = $1 WHERE id = $2`, cover.URL, bookID)
	require.NoError(t, err)

	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	koboSync(t, ts, rawToken)
	require.Eventually(t, func() bool { return hits.Load() == 1 },
		5*time.Second, 10*time.Millisecond)

	koboSync(t, ts, rawToken)
	assert.Never(t, func() bool { return hits.Load() > 1 },
		300*time.Millisecond, 10*time.Millisecond,
		"the second sync joins the running warm")
}

// TestEnsureUserCoverCached_SkipsFilesFoundCoverless: files already found
// coverless aren't downloaded again; a reconverted one is.
func TestEnsureUserCoverCached_SkipsFilesFoundCoverless(t *testing.T) {
	ctx := context.Background()
	owner := "cover-none-" + uuid.NewString()
	epub, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	_, err := testDB.Exec(ctx,
		`UPDATE books.books SET cover_url = NULL WHERE id = $1`, bookID)
	require.NoError(t, err)
	kepub, err := testApp.Services.Conversion.EnsureKEPUB(ctx, owner, bookID)
	require.NoError(t, err)

	cached, err := testApp.Services.Books.EnsureUserCoverCached(ctx, owner, bookID)
	require.NoError(t, err)
	require.False(t, cached)

	for _, key := range []string{epub.StorageKey, kepub.StorageKey} {
		require.NoError(t, fakeStore.Delete(ctx, key))
	}
	cached, err = testApp.Services.Books.EnsureUserCoverCached(ctx, owner, bookID)
	require.NoError(t, err, "coverless files are not downloaded again")
	assert.False(t, cached)

	data := buildCoveredEPUBBytes("Covered")
	require.NoError(t, fakeStore.Put(ctx, kepub.StorageKey,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip"))
	_, err = testDB.Exec(ctx,
		`UPDATE books.book_files SET converter_version = converter_version + 1
		 WHERE id = $1`, kepub.ID)
	require.NoError(t, err)
	cached, err = testApp.Services.Books.EnsureUserCoverCached(ctx, owner, bookID)
	require.NoError(t, err)
	assert.True(t, cached, "a reconverted KEPUB under the same key is reopened")
}

// jpegCover is a JPEG signature, enough for content sniffing.
const jpegCover = "\xff\xd8\xff\xe0epub-cover"

// buildCoveredEPUBBytes is buildEPUBBytes with an EPUB 3 cover-image.
func buildCoveredEPUBBytes(title string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipEntry(zw, "META-INF/container.xml",
		`<?xml version="1.0"?>`+
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"`+
			` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"`+
			` media-type="application/oebps-package+xml"/></rootfiles></container>`,
	)
	writeZipEntry(zw, "OEBPS/content.opf",
		`<?xml version="1.0"?>`+
			`<package xmlns="http://www.idpf.org/2007/opf"`+
			` xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0">`+
			`<metadata><dc:title>`+title+`</dc:title>`+
			`<dc:creator>Author</dc:creator></metadata>`+
			`<manifest><item id="c" href="images/cover.jpg"`+
			` media-type="image/jpeg" properties="cover-image"/></manifest>`+
			`<spine/></package>`,
	)
	writeZipEntry(zw, "OEBPS/images/cover.jpg", jpegCover)
	_ = zw.Close()
	return buf.Bytes()
}

// setupCoveredKoboSyncBook is setupKoboSyncBook for an EPUB carrying a cover
// and a catalog book without a cover URL.
func setupCoveredKoboSyncBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	title := "CoveredKoboBook-" + uuid.NewString()
	seedBookInLibrary(t, ownerID, title, "Author", "")
	result, err := uploadViaTestApp(
		t, ownerID, "covered.epub", buildCoveredEPUBBytes(title),
	)
	require.NoError(t, err)
	bookID := result.UserBook.BookID

	_, err = testDB.Exec(context.Background(),
		`UPDATE books.books SET cover_url = NULL WHERE id = $1`, bookID)
	require.NoError(t, err)
	_, err = testApp.Services.Conversion.EnsureKEPUB(
		context.Background(), ownerID, bookID,
	)
	require.NoError(t, err)

	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), ownerID, bookID,
	))
	return registerTestDevice(t, ownerID), bookID
}

// TestEnableKoboSync_WarmsCoverFromEPUB: a book without a cover URL gets its
// cover from the user's EPUB.
func TestEnableKoboSync_WarmsCoverFromEPUB(t *testing.T) {
	_, bookID := setupCoveredKoboSyncBook(t, "kobo-cover-epub-warm-"+uuid.NewString())

	content, ok := fakeStore.GetContent("books/" + bookID.String() + "/cover.jpg")
	require.True(t, ok)
	assert.Equal(t, jpegCover, string(content))
}

// TestKoboCover_FromEPUB_RevisionedImageID: the handler serves the EPUB's
// cover under the revisioned CoverImageId when nothing is cached yet.
func TestKoboCover_FromEPUB_RevisionedImageID(t *testing.T) {
	owner := "kobo-cover-epub-" + uuid.NewString()
	rawToken, bookID := setupCoveredKoboSyncBook(t, owner)

	coverKey := "books/" + bookID.String() + "/cover.jpg"
	require.NoError(t, fakeStore.Delete(context.Background(), coverKey))

	req := httptest.NewRequest(http.MethodGet,
		"/books/kobo/"+rawToken+"/"+bookID.String()+"-c1/1072/1448/80/isGreyscale/image.jpg",
		nil)
	req.Header.Add("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	getRoutes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), coverKey)
	content, ok := fakeStore.GetContent(coverKey)
	require.True(t, ok)
	assert.Equal(t, jpegCover, string(content))
}
