package backlog_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	bsvc "tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// --- test data helpers ---

func buildEPUBBytes(title, author, isbn13 string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipEntry(zw, "META-INF/container.xml",
		`<?xml version="1.0"?>`+
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"`+
			` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"`+
			` media-type="application/oebps-package+xml"/></rootfiles></container>`,
	)

	var opf strings.Builder
	opf.WriteString(`<?xml version="1.0"?>`)
	opf.WriteString(`<package xmlns="http://www.idpf.org/2007/opf"` +
		` xmlns:dc="http://purl.org/dc/elements/1.1/"` +
		` xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">`)
	opf.WriteString(`<metadata>`)
	fmt.Fprintf(&opf, `<dc:title>%s</dc:title>`, title)
	fmt.Fprintf(&opf, `<dc:creator>%s</dc:creator>`, author)
	if isbn13 != "" {
		fmt.Fprintf(
			&opf,
			`<dc:identifier opf:scheme="ISBN">%s</dc:identifier>`,
			isbn13,
		)
	}
	opf.WriteString(`</metadata></package>`)
	writeZipEntry(zw, "OEBPS/content.opf", opf.String())

	_ = zw.Close()
	return buf.Bytes()
}

func writeZipEntry(zw *zip.Writer, name, content string) {
	w, _ := zw.Create(name)
	_, _ = w.Write([]byte(content))
}

func minimalPDFData() []byte {
	return []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF")
}

// simulateUpload is the two-phase test helper:
//  1. Call CreateUpload to get a presigned key (with empty checksum so no
//     dedup shortcut is applied — ensures bytes always go through R2).
//  2. PUT the bytes into the fake object store at that key.
//  3. Call FinalizeUpload to complete registration.
func simulateUpload(
	ctx context.Context,
	t *testing.T,
	uID string,
	filename, contentType string,
	data []byte,
	fake *objectstore.FakeClient,
) (*bsvc.UploadFileResult, error) {
	t.Helper()
	// Empty checksum forces the slow path (actual upload) for test simplicity.
	uploadID, _, _, err := testApp.Services.Books.CreateUpload(
		ctx, uID, filename, contentType, int64(len(data)), "",
	)
	if err != nil {
		return nil, err
	}
	// Simulate the browser PUT directly to R2.
	require.NoError(
		t,
		fake.Put(ctx, uploadID, bytes.NewReader(data), int64(len(data)), contentType),
	)
	return testApp.Services.Books.FinalizeUpload(
		ctx,
		uID,
		uploadID,
		filename,
		contentType,
		"",
	)
}

// uploadViaTestApp uploads via the shared testApp and fakeStore globals.
func uploadViaTestApp(
	t *testing.T,
	uid, filename, contentType string,
	data []byte,
) (*bsvc.UploadFileResult, error) {
	t.Helper()
	return simulateUpload(
		context.Background(), t, uid, filename, contentType, data, fakeStore,
	)
}

// seedBookInLibrary adds a book with the given title, author, and ISBN13 to the
// specified user's library. Pass an empty string for isbn if the book has none.
// Use this helper whenever recognition must succeed via the title+author or
// ISBN match — ensure the author matches exactly what the EPUB/PDF carries.
func seedBookInLibrary(t *testing.T, uid, title, author, isbn string) *models.UserBook {
	t.Helper()
	cover := "https://example.com/cover.jpg"
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields not needed
		Provider:   "manual",
		ProviderID: fmt.Sprintf("upload-test-%s-%s", title, uuid.New()),
		Title:      title,
		Authors:    []string{author},
		CoverURL:   &cover,
	}
	if isbn != "" {
		ext.ISBN13 = &isbn
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), uid, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// addTestBookForUser seeds a book in the given user's library with the provided
// title and ISBN13 using the standard "Coverage Author" author string.
// Use this in tests that upload as a user other than the global userID.
func addTestBookForUser(t *testing.T, uid, title, isbn string) *models.UserBook {
	t.Helper()
	return seedBookInLibrary(t, uid, title, "Coverage Author", isbn)
}

// --- service-level tests ---

func TestUploadFile_UnsupportedFormat(t *testing.T) {
	fakeStore := fakeStore
	data := []byte("not a recognizable ebook format at all")
	_, err := simulateUpload(
		context.Background(), t, userID, "file.txt", "text/plain", data, fakeStore,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrInvalidFormat)
}

func TestUploadFile_UnsupportedFormat_ShortData(t *testing.T) {
	fakeStore := fakeStore
	_, err := simulateUpload(
		context.Background(), t, userID, "x.bin", "application/octet-stream",
		[]byte{1, 2}, fakeStore,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrInvalidFormat)
}

// TestUploadFile_PDF_NoMetadata_Rejected verifies that a PDF with no Info-dict
// metadata (no title, no author) is rejected because the book cannot be
// recognized, and the temp upload object is removed from the bucket.
func TestUploadFile_PDF_NoMetadata_Rejected(t *testing.T) {
	data := minimalPDFData()
	uploadID, _, _, err := testApp.Services.Books.CreateUpload(
		context.Background(), userID, "no-meta.pdf", "application/pdf",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/pdf",
	))
	_, err = testApp.Services.Books.FinalizeUpload(
		context.Background(), userID, uploadID, "no-meta.pdf", "application/pdf", "",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)

	// Temp object must have been cleaned up by the service.
	exists, existsErr := fakeStore.Exists(context.Background(), uploadID)
	require.NoError(t, existsErr)
	assert.False(t, exists, "temp upload object must be deleted on rejection")
}

func TestUploadFile_EPUB_MatchByISBN(t *testing.T) {
	fakeStore := fakeStore
	const isbn = "9789876543210"
	ub := addTestBookWithISBN(t, "ISBNMatchBook", isbn)

	data := buildEPUBBytes("ISBNMatchBook", "ISBN Author", isbn)
	result, err := simulateUpload(
		context.Background(), t, userID,
		"isbn-match.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.MatchedExisting)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
	assert.Equal(t, models.FileFormatEPUB, result.BookFile.Format)
}

func TestUploadFile_EPUB_MatchByTitleAndAuthor(t *testing.T) {
	fakeStore := fakeStore
	const title = "TitleAuthorMatchUpload"
	ub := addTestBookWithISBN(t, title, "9781111111111")

	data := buildEPUBBytes(title, "Coverage Author", "")
	result, err := simulateUpload(
		context.Background(), t, userID,
		"ta-match.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.MatchedExisting)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

func TestUploadFile_Deduplication(t *testing.T) {
	fakeStore := fakeStore
	addTestBookWithISBN(t, "DedupBook", "9782222222222")
	data := buildEPUBBytes("DedupBook", "Dedup Author", "9782222222222")

	r1, err := simulateUpload(
		context.Background(), t, userID,
		"dedup.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)

	r2, err := simulateUpload(
		context.Background(), t, userID,
		"dedup.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)

	assert.Equal(t, r1.BookFile.ID, r2.BookFile.ID, "second upload returns same file")
}

func TestUploadFile_EnsuresOwnDigitalTag(t *testing.T) {
	fakeStore := fakeStore
	addTestBookWithISBN(t, "OwnDigitalTagBook", "9783333333333")
	data := buildEPUBBytes("OwnDigitalTagBook", "Tag Author", "9783333333333")
	result, err := simulateUpload(
		context.Background(), t, userID,
		"tag-check.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, result.UserBook.BookID,
	)
	require.NoError(t, err)
	assert.Contains(t, ub.Tags, models.TagOwnDigital)
}

// TestUploadFile_CanonicalStorageKey verifies that after finalizeNew the blob
// lives at the content-addressed canonical path (books/<sha256>.ext) instead
// of the temporary upload path.
func TestUploadFile_CanonicalStorageKey(t *testing.T) {
	fakeStore := fakeStore
	ub := addTestBookWithISBN(t, "CanonicalKeyBook", "9780001001001")
	// Remove any stale book_files rows so the upload always goes through
	// finalizeNew, copies the blob, and puts it in the fake store.
	_, _ = testDB.Exec(context.Background(),
		`DELETE FROM backlog.book_files WHERE book_id = $1`, ub.BookID)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE book_id = $1`, ub.BookID)
	})
	data := buildEPUBBytes("CanonicalKeyBook", "Key Author", "9780001001001")
	result, err := simulateUpload(
		context.Background(), t, userID,
		"canonical.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(
		t,
		strings.HasPrefix(result.BookFile.StorageKey, "books/"),
		"storage key must use canonical books/ prefix, got %s",
		result.BookFile.StorageKey,
	)
	assert.NotContains(t, result.BookFile.StorageKey, "uploads/")
	// The canonical blob must exist in the fake store.
	exists, err := fakeStore.Exists(
		context.Background(), result.BookFile.StorageKey,
	)
	require.NoError(t, err)
	assert.True(t, exists, "canonical blob should be present in the store")
}

// TestUploadFile_GlobalDedup_SameUser_CreateShortcut verifies that when the
// client provides the correct checksum in CreateBookUpload and the content is
// already stored, the server returns already_exists=true so the PUT is skipped.
func TestUploadFile_GlobalDedup_SameUser_CreateShortcut(t *testing.T) {
	addTestBookWithISBN(t, "GlobalDedupCSBook", "9780001002002")
	// First upload via the slow path (no checksum).
	data := buildEPUBBytes("GlobalDedupCSBook", "GD Author", "9780001002002")
	r1, err := simulateUpload(
		context.Background(), t, userID,
		"dedup-cs.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, r1)

	// Second CreateUpload call with the correct checksum — must report already_exists.
	checksum := *r1.BookFile.Checksum
	_, _, alreadyExists, err := testApp.Services.Books.CreateUpload(
		context.Background(), userID, "dedup-cs.epub",
		"application/epub+zip", int64(len(data)), checksum,
	)
	require.NoError(t, err)
	assert.True(
		t,
		alreadyExists,
		"server should report already_exists for known checksum",
	)
}

// TestUploadFile_GlobalDedup_CrossUser verifies that uploading the same file
// as a different user reuses the canonical blob without creating a second R2
// object, but creates a separate book_files row owned by user B.
func TestUploadFile_GlobalDedup_CrossUser(t *testing.T) {
	const userB = "cross-user-dedup-user-b"
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, userB)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, userB)
	})

	ub := addTestBookWithISBN(t, "CrossUserBook", "9780001003003")
	// Remove stale book_files so both users always get fresh rows in this run.
	_, _ = testDB.Exec(context.Background(),
		`DELETE FROM backlog.book_files WHERE book_id = $1`, ub.BookID)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE book_id = $1`, ub.BookID)
	})
	data := buildEPUBBytes("CrossUserBook", "Cross Author", "9780001003003")

	// User A uploads first.
	r1, err := simulateUpload(
		context.Background(), t, userID,
		"cross-user.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, r1)
	canonicalKey := r1.BookFile.StorageKey
	require.True(t, strings.HasPrefix(canonicalKey, "books/"),
		"user A's blob must be at canonical key")

	// User B finalizes with the same checksum, no upload.
	checksum := *r1.BookFile.Checksum
	r2, err := testApp.Services.Books.FinalizeUpload(
		context.Background(), userB, "", "cross-user.epub", "application/epub+zip",
		checksum,
	)
	require.NoError(t, err)
	require.NotNil(t, r2)

	// User B gets their own book_files row.
	assert.Equal(t, userB, r2.BookFile.UserID)
	// But the physical blob is the same canonical object.
	assert.Equal(t, canonicalKey, r2.BookFile.StorageKey)
	// File IDs are distinct.
	assert.NotEqual(t, r1.BookFile.ID, r2.BookFile.ID)

	// Only one canonical blob exists in the store.
	_, existsA := fakeStore.GetContent(canonicalKey)
	assert.True(t, existsA, "canonical blob must still exist")
}

// TestUploadFile_GlobalDedup_CrossUser_ClearLibraryPreservesBlob verifies that
// clearing user A's library does NOT delete the canonical blob while user B
// still references it, and that clearing both eventually removes the blob.
func TestUploadFile_GlobalDedup_CrossUser_ClearLibraryPreservesBlob(t *testing.T) {
	const userA2 = "refcount-user-a2"
	const userB2 = "refcount-user-b2"
	t.Cleanup(func() {
		for _, uid := range []string{userA2, userB2} {
			_, _ = testDB.Exec(context.Background(),
				`DELETE FROM backlog.user_books WHERE user_id = $1`, uid)
			_, _ = testDB.Exec(context.Background(),
				`DELETE FROM backlog.book_files WHERE user_id = $1`, uid)
		}
	})

	addTestBookForUser(t, userA2, "RefCountBook", "9780001004004")
	data := buildEPUBBytes("RefCountBook", "Refcount Author", "9780001004004")

	// User A2 uploads first.
	r1, err := simulateUpload(
		context.Background(), t, userA2,
		"refcount.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	canonicalKey := r1.BookFile.StorageKey

	// User B2 attaches to the same canonical blob.
	checksum := *r1.BookFile.Checksum
	_, err = testApp.Services.Books.FinalizeUpload(
		context.Background(), userB2, "", "refcount.epub", "application/epub+zip",
		checksum,
	)
	require.NoError(t, err)

	// Clear user A2. Blob must survive because B2 still references it.
	_, _, err = testApp.Services.Books.ClearLibrary(context.Background(), userA2)
	require.NoError(t, err)
	exists, err := fakeStore.Exists(context.Background(), canonicalKey)
	require.NoError(t, err)
	assert.True(
		t,
		exists,
		"canonical blob must survive while userB2 still references it",
	)

	// Clear user B2. Now the blob must be gone.
	_, _, err = testApp.Services.Books.ClearLibrary(context.Background(), userB2)
	require.NoError(t, err)
	exists, err = fakeStore.Exists(context.Background(), canonicalKey)
	require.NoError(t, err)
	assert.False(
		t,
		exists,
		"canonical blob must be deleted when the last reference is gone",
	)
}

// --- repository tests ---

func TestBooksRepo_FindUserBookByISBN13_Found(t *testing.T) {
	ub := addTestBookWithISBN(t, "ISBNRepoFind", "9784444444444")

	got, err := testApp.Repositories.Books.FindUserBookByISBN13(
		context.Background(), userID, "9784444444444",
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, ub.BookID, got.BookID)
}

func TestBooksRepo_FindUserBookByISBN13_NotFound(t *testing.T) {
	got, err := testApp.Repositories.Books.FindUserBookByISBN13(
		context.Background(), userID, "9780000000000",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBooksRepo_FindUserBookByTitleAndAuthor_Found(t *testing.T) {
	ub := addTestBookWithISBN(t, "TitleAuthorRepoFind", "9785555555555")

	got, err := testApp.Repositories.Books.FindUserBookByTitleAndAuthor(
		context.Background(), userID, "TitleAuthorRepoFind", "Coverage Author",
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, ub.BookID, got.BookID)
}

func TestBooksRepo_FindUserBookByTitleAndAuthor_NotFound(t *testing.T) {
	got, err := testApp.Repositories.Books.FindUserBookByTitleAndAuthor(
		context.Background(), userID, "NoSuchTitleXYZ", "NoSuchAuthorXYZ",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_FindByChecksum_Found(t *testing.T) {
	book := addUniqueBook(t)
	chk := "sha256testchecksum"
	f := models.BookFile{ //nolint:exhaustruct //optional fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatEPUB,
		StorageKey: "books/sha256testchecksum.epub",
		SizeBytes:  512,
		Checksum:   &chk,
		Status:     models.FileStatusReady,
	}
	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.FindByChecksum(
		context.Background(), userID, book.ID, models.FileFormatEPUB, chk,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
}

func TestBookFilesRepo_FindByChecksum_NotFound(t *testing.T) {
	book := addUniqueBook(t)
	got, err := testApp.Repositories.BookFiles.FindByChecksum(
		context.Background(), userID, book.ID, models.FileFormatEPUB, "nonexistent",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_FindByChecksumGlobal_Found(t *testing.T) {
	book := addUniqueBook(t)
	const chk = "globalchecksumABC"
	f := models.BookFile{ //nolint:exhaustruct //optional fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatEPUB,
		StorageKey: "books/" + chk + ".epub",
		SizeBytes:  512,
		Checksum:   func() *string { s := chk; return &s }(),
		Status:     models.FileStatusReady,
	}
	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.FindByChecksumGlobal(
		context.Background(), chk,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
}

func TestBookFilesRepo_FindByChecksumGlobal_NotFound(t *testing.T) {
	got, err := testApp.Repositories.BookFiles.FindByChecksumGlobal(
		context.Background(), "no-such-global-checksum-xyz",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_CountByStorageKey(t *testing.T) {
	book := addUniqueBook(t)
	// Use a unique key per test run so repeated runs on the shared DB don't
	// accumulate stale rows from previous executions.
	key := fmt.Sprintf("books/count-test-%s.epub", book.ID)
	// Start: key has no references.
	n, err := testApp.Repositories.BookFiles.CountByStorageKey(
		context.Background(), key,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

	// Insert two rows pointing at the same key.
	for i := range 2 {
		chk := fmt.Sprintf("count-chk-%d", i)
		_, insertErr := testApp.Repositories.BookFiles.Insert(
			context.Background(),
			models.BookFile{ //nolint:exhaustruct //optional fields omitted
				BookID:     book.ID,
				UserID:     fmt.Sprintf("count-user-%d", i),
				Format:     models.FileFormatEPUB,
				StorageKey: key,
				SizeBytes:  256,
				Checksum:   &chk,
				Status:     models.FileStatusReady,
			},
		)
		require.NoError(t, insertErr)
	}

	n, err = testApp.Repositories.BookFiles.CountByStorageKey(
		context.Background(), key,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// TestUploadFile_EPUB_HardcoverFallback covers the Hardcover search branch.
func TestUploadFile_EPUB_HardcoverFallback(t *testing.T) {
	const isolatedUser = "hc-fallback-upload-user"

	app2 := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client { return nil },
			HardcoverFactory: func(_ string) hardcover.Client {
				return mocks.NewMockHardcoverClient()
			},
			ObjectStore:      fakeStore,
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{}, //nolint:exhaustruct //hardcover key comes from config
	))
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
	})

	data := buildEPUBBytes("No Match Title", "No Match Author HC", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "hc-fallback.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))
	result, err := app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID,
		"hc-fallback.epub", "application/epub+zip", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.MatchedExisting)
	assert.NotEmpty(t, result.UserBook.BookID)
}

// --- security hardening tests ---

func TestUploadFile_WrongMagicBytes_Rejected(t *testing.T) {
	fakeStore := fakeStore
	data := append([]byte("\x00\x01\x02\x03"), []byte("not a real epub")...)
	_, err := simulateUpload(
		context.Background(), t, userID,
		"legit-looking.epub", "application/epub+zip", data, fakeStore,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrInvalidFormat)
}

func TestUploadFile_WrongMagicBytes_PDF_Rejected(t *testing.T) {
	fakeStore := fakeStore
	data := []byte("\xFF\xFE content-type says pdf but magic says no")
	_, err := simulateUpload(
		context.Background(), t, userID,
		"fake.pdf", "application/pdf", data, fakeStore,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrInvalidFormat)
}

func TestUploadFile_OverSize_Rejected(t *testing.T) {
	// CreateUpload checks size before any bytes are transferred.
	_, _, _, err := testApp.Services.Books.CreateUpload(
		context.Background(), userID, "huge.epub", "application/epub+zip",
		int64(bsvc.MaxUploadBytes)+1, "",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrFileTooLarge)
}

func TestUploadFile_OwnershipRejected(t *testing.T) {
	_, err := testApp.Services.Books.FinalizeUpload(
		context.Background(), userID,
		"users/other-user/uploads/uuid.epub",
		"file.epub", "application/epub+zip", "",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrInvalidUploadID)
}

func TestUploadFile_FilenameHygiene_LongNameTruncated(t *testing.T) {
	fakeStore := fakeStore
	addTestBookWithISBN(t, "LongNameBook", "9787777777777")
	longName := strings.Repeat("a", 300) + ".epub"
	data := buildEPUBBytes("LongNameBook", "Long Author", "9787777777777")
	result, err := simulateUpload(
		context.Background(),
		t,
		userID,
		longName,
		"application/epub+zip",
		data,
		fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result.BookFile.OriginalFilename)
	assert.LessOrEqual(t, len(*result.BookFile.OriginalFilename), 255)
}

func TestUploadFile_FilenameHygiene_PathTraversalStoredSafely(t *testing.T) {
	fakeStore := fakeStore
	addTestBookWithISBN(t, "PathTraversal Book", "9786543210987")
	data := buildEPUBBytes("PathTraversal Book", "Path Author", "9786543210987")
	result, err := simulateUpload(
		context.Background(), t, userID,
		"../../etc/passwd", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.BookFile.StorageKey, "..")
	assert.NotContains(t, result.BookFile.StorageKey, "etc/passwd")
}

func TestUploadFile_OwnershipFromContext(t *testing.T) {
	fakeStore := fakeStore
	addTestBookWithISBN(t, "OwnershipBook", "9781234567890")
	data := buildEPUBBytes("OwnershipBook", "Owner Author", "9781234567890")
	result, err := simulateUpload(
		context.Background(), t, userID,
		"ownership.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	// The book_files row must be owned by the requesting user.
	assert.Equal(t, userID, result.BookFile.UserID)
	// The blob is stored at the canonical content-addressed path, not under
	// a per-user prefix — that is expected and correct.
	assert.True(
		t,
		strings.HasPrefix(result.BookFile.StorageKey, "books/"),
		"canonical key must start with books/, got %s",
		result.BookFile.StorageKey,
	)
}

// --- handler tests ---

func TestConnectCreateBookUpload_OK(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "handler-test.epub",
		ContentType: "application/epub+zip",
		Size:        1024,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.CreateBookUpload(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.UploadId)
	assert.NotEmpty(t, resp.Msg.Url)
	assert.Contains(t, resp.Msg.UploadId, "users/"+userID+"/uploads/")
	assert.False(t, resp.Msg.AlreadyExists)
}

func TestConnectCreateBookUpload_Oversize_ReturnsResourceExhausted(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "huge.epub",
		ContentType: "application/epub+zip",
		Size:        int64(bsvc.MaxUploadBytes) + 1,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.CreateBookUpload(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeResourceExhausted, connectErr.Code())
}

func TestConnectFinalizeBookUpload_OK(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addTestBookWithISBN(t, "HandlerUploadBook", "9786666666666")
	data := buildEPUBBytes("HandlerUploadBook", "Handler Author", "9786666666666")
	createReq := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "handler-test.epub",
		ContentType: "application/epub+zip",
		Size:        int64(len(data)),
	})
	createReq.Header().Set("Cookie", accessToken.String())
	createResp, err := client.CreateBookUpload(ctx, createReq)
	require.NoError(t, err)

	require.NoError(t, fakeStore.Put(
		ctx, createResp.Msg.UploadId,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	finalReq := connect.NewRequest(&backlogv1.FinalizeBookUploadRequest{
		UploadId:    createResp.Msg.UploadId,
		Filename:    "handler-test.epub",
		ContentType: "application/epub+zip",
	})
	finalReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.FinalizeBookUpload(ctx, finalReq)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.BookId)
	assert.NotEmpty(t, resp.Msg.FileId)
	assert.Equal(t, models.FileFormatEPUB, resp.Msg.Format)
}

// TestConnectFinalizeBookUpload_PDF_NoMetadata verifies that a PDF with no
// Info-dict metadata is rejected with CodeInvalidArgument.
func TestConnectFinalizeBookUpload_PDF_NoMetadata(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := minimalPDFData()
	createReq := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "handler-pdf.pdf",
		ContentType: "application/pdf",
		Size:        int64(len(data)),
	})
	createReq.Header().Set("Cookie", accessToken.String())
	createResp, err := client.CreateBookUpload(ctx, createReq)
	require.NoError(t, err)

	require.NoError(t, fakeStore.Put(
		ctx, createResp.Msg.UploadId,
		bytes.NewReader(data), int64(len(data)), "application/pdf",
	))

	finalReq := connect.NewRequest(&backlogv1.FinalizeBookUploadRequest{
		UploadId:    createResp.Msg.UploadId,
		Filename:    "handler-pdf.pdf",
		ContentType: "application/pdf",
	})
	finalReq.Header().Set("Cookie", accessToken.String())
	_, err = client.FinalizeBookUpload(ctx, finalReq)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestConnectFinalizeBookUpload_WrongMagicBytes(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := []byte("\x00\x01\x02\x03 not epub magic")
	createReq := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "evil.epub",
		ContentType: "application/epub+zip",
		Size:        int64(len(data)),
	})
	createReq.Header().Set("Cookie", accessToken.String())
	createResp, err := client.CreateBookUpload(ctx, createReq)
	require.NoError(t, err)

	require.NoError(t, fakeStore.Put(
		ctx, createResp.Msg.UploadId,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	finalReq := connect.NewRequest(&backlogv1.FinalizeBookUploadRequest{
		UploadId:    createResp.Msg.UploadId,
		Filename:    "evil.epub",
		ContentType: "application/epub+zip",
	})
	finalReq.Header().Set("Cookie", accessToken.String())
	_, err = client.FinalizeBookUpload(ctx, finalReq)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestConnectFinalizeBookUpload_WrongOwner_ReturnsPermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finalReq := connect.NewRequest(&backlogv1.FinalizeBookUploadRequest{
		UploadId:    "users/other-user/uploads/uuid.epub",
		Filename:    "stolen.epub",
		ContentType: "application/epub+zip",
	})
	finalReq.Header().Set("Cookie", accessToken.String())
	_, err := client.FinalizeBookUpload(ctx, finalReq)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

// TestUploadFile_Unrecognized_EmptyMetadata_Rejected uploads an EPUB whose OPF
// metadata has empty title, author, and no ISBN. With no library match and no
// Hardcover client, the service must return ErrUnrecognizedBook and clean up
// the temp upload object.
func TestUploadFile_Unrecognized_EmptyMetadata_Rejected(t *testing.T) {
	data := buildEPUBBytes("", "", "")
	uploadID, _, _, err := testApp.Services.Books.CreateUpload(
		context.Background(), userID, "empty.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	_, err = testApp.Services.Books.FinalizeUpload(
		context.Background(),
		userID,
		uploadID,
		"empty.epub",
		"application/epub+zip",
		"",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)

	// Temp object must have been cleaned up by the service.
	exists, existsErr := fakeStore.Exists(context.Background(), uploadID)
	require.NoError(t, existsErr)
	assert.False(t, exists, "temp upload object must be deleted on rejection")
}

// noHardcoverApp returns an isolated Backlog instance with no Hardcover API key
// set, so SearchHardcover always returns nil. Used to test the rejection path
// when neither a library match nor a Hardcover match is found.
func noHardcoverApp(t *testing.T, isolatedUser string) *backlog.Backlog {
	t.Helper()
	cfg := testCfg
	cfg.HardcoverAPIKey = ""
	return backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		cfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return nil },
			HardcoverFactory: func(_ string) hardcover.Client { return nil },
			ObjectStore:      fakeStore,
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
}

// TestUploadFile_Unrecognized_NoLibraryMatch_Rejected uploads an EPUB that has
// valid title/author metadata but is not in the library and the app has no
// Hardcover key configured. The upload must be rejected with ErrUnrecognizedBook.
func TestUploadFile_Unrecognized_NoLibraryMatch_Rejected(t *testing.T) {
	const isolatedUser = "no-match-upload-user"
	app2 := noHardcoverApp(t, isolatedUser)

	data := buildEPUBBytes("NoLibraryMatchTitle", "NoLibraryMatchAuthor", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "no-match.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	_, err = app2.Services.Books.FinalizeUpload(
		context.Background(),
		isolatedUser,
		uploadID,
		"no-match.epub",
		"application/epub+zip",
		"",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)
}

// --- ISBN10 matching tests ---

// seedBookWithISBN10 adds a book that carries an ISBN10 (no ISBN13) to the
// specified user's library. The library lookup for the upload must use
// FindUserBookByISBN10 to link the file.
func seedBookWithISBN10(
	t *testing.T,
	uid, title, author, isbn10 string,
) *models.UserBook {
	t.Helper()
	cover := "https://example.com/cover.jpg"
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields not needed
		Provider:   "manual",
		ProviderID: fmt.Sprintf("isbn10-test-%s-%s", title, uuid.New()),
		Title:      title,
		Authors:    []string{author},
		ISBN10:     &isbn10,
		CoverURL:   &cover,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), uid, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

func TestBooksRepo_FindUserBookByISBN10_Found(t *testing.T) {
	// Use an isolated user so the isbn10 value is unique in the DB for this test.
	const isolatedUser = "isbn10-repo-find-user"
	const isbn10 = "0140449116"
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
	})

	ub := seedBookWithISBN10(t, isolatedUser, "FindISBN10Book", "ISBN10 Author", isbn10)

	got, err := testApp.Repositories.Books.FindUserBookByISBN10(
		context.Background(), isolatedUser, isbn10,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, ub.BookID, got.BookID)
}

func TestBooksRepo_FindUserBookByISBN10_NotFound(t *testing.T) {
	got, err := testApp.Repositories.Books.FindUserBookByISBN10(
		context.Background(), userID, "0000000000",
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

// TestUploadFile_EPUB_MatchByISBN10 verifies that an EPUB whose only identifier
// is an ISBN10 links to an existing library entry that carries that ISBN10.
// Uses an isolated user to avoid cross-test isbn10 collisions.
func TestUploadFile_EPUB_MatchByISBN10(t *testing.T) {
	const isolatedUser = "isbn10-match-upload-user"
	// isbn10 must be exactly 10 digits so classifyISBN detects it as ISBN10.
	// Use a value unique to this test to avoid shared-DB collisions.
	const isbn10 = "0062459961"
	app2 := noHardcoverApp(t, isolatedUser)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, isolatedUser)
	})

	ub := seedBookInLibrary(t, isolatedUser, "ISBN10MatchBook", "ISBN10 Author", "")
	// Write isbn10 directly onto the book so FindUserBookByISBN10 can find it.
	_, err := testDB.Exec(context.Background(),
		`UPDATE backlog.books SET isbn10 = $1 WHERE id = $2`,
		isbn10, ub.BookID,
	)
	require.NoError(t, err)

	// buildEPUBBytes embeds the identifier via dc:identifier; classifyISBN
	// will detect a 10-character string as ISBN10.
	data := buildEPUBBytes("ISBN10MatchBook", "ISBN10 Author", isbn10)
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "isbn10-match.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	result, err := app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID,
		"isbn10-match.epub", "application/epub+zip", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.MatchedExisting)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

// --- normalized title / author matching tests ---

// TestUploadFile_EPUB_MatchByNormalizedTitle_Subtitle verifies that a file
// carrying "Title: Subtitle" links to a library entry that has only "Title".
func TestUploadFile_EPUB_MatchByNormalizedTitle_Subtitle(t *testing.T) {
	const isolatedUser = "norm-title-subtitle-user"
	app2 := noHardcoverApp(t, isolatedUser)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, isolatedUser)
	})

	// Library has "The Silmarillion" without the subtitle.
	ub := seedBookInLibrary(
		t, isolatedUser,
		"The Silmarillion", "J.R.R. Tolkien", "",
	)

	// EPUB carries the full title with subtitle.
	data := buildEPUBBytes(
		"The Silmarillion: Being the Myths and Legends of the First Age",
		"J.R.R. Tolkien",
		"",
	)
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "silm.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	result, err := app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID,
		"silm.epub", "application/epub+zip", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.MatchedExisting, "subtitle mismatch must still link")
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

// TestUploadFile_EPUB_MatchByNormalizedAuthor_LastFirst verifies that a file
// whose author is formatted "Last, First" links to a library entry with
// "First Last" formatting.
func TestUploadFile_EPUB_MatchByNormalizedAuthor_LastFirst(t *testing.T) {
	const isolatedUser = "norm-author-lastfirst-user"
	app2 := noHardcoverApp(t, isolatedUser)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, isolatedUser)
	})

	// Library has "First Last" author format.
	ub := seedBookInLibrary(
		t, isolatedUser, "The Two Towers", "J.R.R. Tolkien", "",
	)

	// EPUB carries "Last, First" author format.
	data := buildEPUBBytes("The Two Towers", "Tolkien, J.R.R.", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "ttt.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	result, err := app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID,
		"ttt.epub", "application/epub+zip", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(
		t, result.MatchedExisting,
		"last-comma-first author format must still link",
	)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

// TestUploadFile_EPUB_NormalizedMatch_DifferentAuthor_NoFalsePositive verifies
// that same-title books by different authors are NOT linked incorrectly.
func TestUploadFile_EPUB_NormalizedMatch_DifferentAuthor_NoFalsePositive(t *testing.T) {
	const isolatedUser = "norm-false-positive-user"
	app2 := noHardcoverApp(t, isolatedUser)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.book_files WHERE user_id = $1`, isolatedUser)
	})

	// Library has "Hamlet" by Shakespeare — not by Orwell.
	seedBookInLibrary(t, isolatedUser, "Hamlet", "William Shakespeare", "")

	// File claims "Hamlet" by George Orwell — must NOT link.
	data := buildEPUBBytes("Hamlet", "George Orwell", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "hamlet.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	_, err = app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID,
		"hamlet.epub", "application/epub+zip", "",
	)
	// With no Hardcover and no match, expect ErrUnrecognizedBook.
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook,
		"different author must not create a false-positive link")
}

// TestConnectFinalizeBookUpload_Unrecognized_ReturnsInvalidArgument verifies
// that the ConnectRPC handler maps ErrUnrecognizedBook to CodeInvalidArgument.
func TestConnectFinalizeBookUpload_Unrecognized_ReturnsInvalidArgument(t *testing.T) {
	const isolatedUser = "handler-unrecognized-upload-user"
	app2 := noHardcoverApp(t, isolatedUser)

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := backlogv1connect.NewBooksServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := buildEPUBBytes("HandlerUnrecognizedBook", "Unknown Author Handler", "")
	createReq := connect.NewRequest(&backlogv1.CreateBookUploadRequest{
		Filename:    "unrecognized.epub",
		ContentType: "application/epub+zip",
		Size:        int64(len(data)),
	})
	createReq.Header().Set("Cookie", accessToken.String())
	createResp, err := client.CreateBookUpload(ctx, createReq)
	require.NoError(t, err)

	require.NoError(t, fakeStore.Put(
		ctx, createResp.Msg.UploadId,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	finalReq := connect.NewRequest(&backlogv1.FinalizeBookUploadRequest{
		UploadId:    createResp.Msg.UploadId,
		Filename:    "unrecognized.epub",
		ContentType: "application/epub+zip",
	})
	finalReq.Header().Set("Cookie", accessToken.String())
	_, err = client.FinalizeBookUpload(ctx, finalReq)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
