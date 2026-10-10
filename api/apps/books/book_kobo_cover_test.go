package books_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
