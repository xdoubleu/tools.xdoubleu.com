package books_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	bsvc "tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/database"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

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

// simulateUpload runs CreateUpload (empty checksum, so bytes always go through
// R2), PUTs into the fake store, then FinalizeUpload.
func simulateUpload(
	ctx context.Context,
	t *testing.T,
	uID string,
	filename, contentType string,
	data []byte,
	fake *objectstore.FakeClient,
) (*bsvc.UploadFileResult, error) {
	t.Helper()
	uploadID, _, _, err := testApp.Services.Books.CreateUpload(
		ctx, uID, filename, contentType, int64(len(data)), "",
	)
	if err != nil {
		return nil, err
	}
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
		"",
		"",
	)
}

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

// seedBookInLibrary adds a book to uid's library (isbn may be ""). The author
// must match the file's exactly for recognition.
func seedBookInLibrary(t *testing.T, uid, title, author, isbn string) *models.UserBook {
	t.Helper()
	ext := bsvc.SourceProposal{ //nolint:exhaustruct //optional fields not needed
		Source:   "manual",
		Title:    title,
		Authors:  []string{author},
		CoverURL: "https://example.com/cover.jpg",
	}
	if isbn != "" {
		ext.ISBN13 = isbn
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), uid, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

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

// TestUploadFile_PDF_NoMetadata_Rejected: an unrecognizable PDF is rejected
// and its temp object removed. Uses noExternalMatchApp since testApp's mock
// Hardcover matches any query.
func TestUploadFile_PDF_NoMetadata_Rejected(t *testing.T) {
	const isolatedUser = "pdf-no-meta-user"
	app2 := noExternalMatchApp(t, isolatedUser)

	data := minimalPDFData()
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "no-meta.pdf", "application/pdf",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/pdf",
	))
	_, err = app2.Services.Books.FinalizeUpload(
		context.Background(), isolatedUser, uploadID, "no-meta.pdf",
		"application/pdf", "", "", "",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)

	exists, existsErr := fakeStore.Exists(context.Background(), uploadID)
	require.NoError(t, existsErr)
	assert.False(t, exists, "temp upload object must be deleted on rejection")
}

// TestUploadFile_PDF_NoMetadata_FilenameFallback_Matches checks the
// filename-derived title resolves a match.
func TestUploadFile_PDF_NoMetadata_FilenameFallback_Matches(t *testing.T) {
	fakeStore := fakeStore
	data := minimalPDFData()
	result, err := simulateUpload(
		context.Background(), t, userID,
		"Black Hat Go.pdf", "application/pdf", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.MatchedExisting)
}

// TestUploadFile_PDF_NoMetadata_TitleOverride_Matches checks an explicit title
// override resolves a match when the filename is unusable.
func TestUploadFile_PDF_NoMetadata_TitleOverride_Matches(t *testing.T) {
	data := minimalPDFData()
	uploadID, _, _, err := testApp.Services.Books.CreateUpload(
		context.Background(), userID, "learnyousomeerlangforgreatgood.pdf",
		"application/pdf", int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/pdf",
	))
	result, err := testApp.Services.Books.FinalizeUpload(
		context.Background(), userID, uploadID,
		"learnyousomeerlangforgreatgood.pdf", "application/pdf", "",
		"Learn You Some Erlang for Great Good", "Fred Hébert",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.MatchedExisting)
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

// TestUploadFile_CanonicalStorageKey checks the blob moves to its
// content-addressed key.
func TestUploadFile_CanonicalStorageKey(t *testing.T) {
	fakeStore := fakeStore
	ub := addTestBookWithISBN(t, "CanonicalKeyBook", "9780001001001")
	// Clear stale rows so the upload goes through finalizeNew.
	_, _ = testDB.Exec(context.Background(),
		`DELETE FROM books.book_files WHERE book_id = $1`, ub.BookID)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE book_id = $1`, ub.BookID)
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
	exists, err := fakeStore.Exists(
		context.Background(), result.BookFile.StorageKey,
	)
	require.NoError(t, err)
	assert.True(t, exists, "canonical blob should be present in the store")
}

// TestUploadFile_GlobalDedup_SameUser_CreateShortcut checks a known checksum
// returns already_exists.
func TestUploadFile_GlobalDedup_SameUser_CreateShortcut(t *testing.T) {
	addTestBookWithISBN(t, "GlobalDedupCSBook", "9780001002002")
	data := buildEPUBBytes("GlobalDedupCSBook", "GD Author", "9780001002002")
	r1, err := simulateUpload(
		context.Background(), t, userID,
		"dedup-cs.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, r1)

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

// TestUploadFile_GlobalDedup_CrossUser: another user's upload reuses the blob
// but gets its own book_files row.
func TestUploadFile_GlobalDedup_CrossUser(t *testing.T) {
	const userB = "cross-user-dedup-user-b"
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1`, userB)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE user_id = $1`, userB)
	})

	ub := addTestBookWithISBN(t, "CrossUserBook", "9780001003003")
	_, _ = testDB.Exec(context.Background(),
		`DELETE FROM books.book_files WHERE book_id = $1`, ub.BookID)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE book_id = $1`, ub.BookID)
	})
	data := buildEPUBBytes("CrossUserBook", "Cross Author", "9780001003003")

	r1, err := simulateUpload(
		context.Background(), t, userID,
		"cross-user.epub", "application/epub+zip", data, fakeStore,
	)
	require.NoError(t, err)
	require.NotNil(t, r1)
	canonicalKey := r1.BookFile.StorageKey
	require.True(t, strings.HasPrefix(canonicalKey, "books/"),
		"user A's blob must be at canonical key")

	checksum := *r1.BookFile.Checksum
	r2, err := testApp.Services.Books.FinalizeUpload(
		context.Background(), userB, "", "cross-user.epub", "application/epub+zip",
		checksum, "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, r2)

	assert.Equal(t, userB, r2.BookFile.UserID)
	assert.Equal(t, canonicalKey, r2.BookFile.StorageKey)
	assert.NotEqual(t, r1.BookFile.ID, r2.BookFile.ID)

	_, existsA := fakeStore.GetContent(canonicalKey)
	assert.True(t, existsA, "canonical blob must still exist")
}

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
	// Unique per run: FindByChecksumGlobal would match a previous run's row.
	chk := fmt.Sprintf("globalchecksum-%s", book.ID)
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
	// Unique per run so reruns on the shared DB don't accumulate rows.
	key := fmt.Sprintf("books/count-test-%s.epub", book.ID)
	n, err := testApp.Repositories.BookFiles.CountByStorageKey(
		context.Background(), key,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

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

// TestUploadFile_EPUB_ExternalSearchFallback covers the external search branch.
func TestUploadFile_EPUB_ExternalSearchFallback(t *testing.T) {
	const isolatedUser = "hc-fallback-upload-user"

	app2 := books.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Hardcover:        mocks.NewMockHardcoverClient(),
			ObjectStore:      fakeStore,
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1`, isolatedUser)
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
		"hc-fallback.epub", "application/epub+zip", "", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.MatchedExisting)
	assert.NotEmpty(t, result.UserBook.BookID)
}

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
		"file.epub", "application/epub+zip", "", "", "",
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
	assert.Equal(t, userID, result.BookFile.UserID)
	assert.True(
		t,
		strings.HasPrefix(result.BookFile.StorageKey, "books/"),
		"canonical key must start with books/, got %s",
		result.BookFile.StorageKey,
	)
}

func TestConnectCreateBookUpload_OK(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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

	req := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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
	createReq := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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

	finalReq := connect.NewRequest(&booksv1.FinalizeBookUploadRequest{
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

// TestConnectFinalizeBookUpload_PDF_NoMetadata checks an unrecognizable PDF
// yields CodeInvalidArgument.
func TestConnectFinalizeBookUpload_PDF_NoMetadata(t *testing.T) {
	const isolatedUser = "handler-pdf-no-meta-user"
	app2 := noExternalMatchApp(t, isolatedUser)
	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := newBooksClientFor(ts.URL, connect.WithHTTPGet())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := minimalPDFData()
	createReq := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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

	finalReq := connect.NewRequest(&booksv1.FinalizeBookUploadRequest{
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
	createReq := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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

	finalReq := connect.NewRequest(&booksv1.FinalizeBookUploadRequest{
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

	finalReq := connect.NewRequest(&booksv1.FinalizeBookUploadRequest{
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

// TestUploadFile_Unrecognized_EmptyMetadata_Rejected checks an EPUB with empty
// metadata is rejected and its temp object removed.
func TestUploadFile_Unrecognized_EmptyMetadata_Rejected(t *testing.T) {
	const isolatedUser = "empty-metadata-user"
	app2 := noExternalMatchApp(t, isolatedUser)

	data := buildEPUBBytes("", "", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), isolatedUser, "empty.epub", "application/epub+zip",
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
		"empty.epub",
		"application/epub+zip",
		"",
		"",
		"",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)

	exists, existsErr := fakeStore.Exists(context.Background(), uploadID)
	require.NoError(t, existsErr)
	assert.False(t, exists, "temp upload object must be deleted on rejection")
}

// noExternalMatchApp returns an isolated app whose Hardcover client finds nothing.
func noExternalMatchApp(t *testing.T, isolatedUser string) *books.Books {
	t.Helper()
	return books.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		books.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Hardcover:        mocks.NewMockEmptyHardcoverClient(),
			ObjectStore:      fakeStore,
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
}

// TestUploadFile_Unrecognized_NoLibraryMatch_Rejected checks an unmatched EPUB
// is rejected.
func TestUploadFile_Unrecognized_NoLibraryMatch_Rejected(t *testing.T) {
	const isolatedUser = "no-match-upload-user"
	app2 := noExternalMatchApp(t, isolatedUser)

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
		"",
		"",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook)
}

// TestUploadFile_EPUB_MatchByNormalizedTitle_Subtitle: "Title: Subtitle"
// matches a library "Title".
func TestUploadFile_EPUB_MatchByNormalizedTitle_Subtitle(t *testing.T) {
	const isolatedUser = "norm-title-subtitle-user"
	app2 := noExternalMatchApp(t, isolatedUser)

	ub := seedBookInLibrary(
		t, isolatedUser,
		"The Silmarillion", "J.R.R. Tolkien", "",
	)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE user_id = $1`, isolatedUser)
		// Matching is catalog-wide, so clear leftover catalog rows from prior runs.
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.books WHERE id = $1`, ub.BookID)
	})

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
		"silm.epub", "application/epub+zip", "", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.MatchedExisting, "subtitle mismatch must still link")
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

// TestUploadFile_EPUB_MatchByNormalizedAuthor_LastFirst: "Last, First" matches
// "First Last".
func TestUploadFile_EPUB_MatchByNormalizedAuthor_LastFirst(t *testing.T) {
	const isolatedUser = "norm-author-lastfirst-user"
	app2 := noExternalMatchApp(t, isolatedUser)

	ub := seedBookInLibrary(
		t, isolatedUser, "The Two Towers", "J.R.R. Tolkien", "",
	)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE user_id = $1`, isolatedUser)
		// Matching is catalog-wide, so clear leftover catalog rows from prior runs.
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.books WHERE id = $1`, ub.BookID)
	})

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
		"ttt.epub", "application/epub+zip", "", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(
		t, result.MatchedExisting,
		"last-comma-first author format must still link",
	)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
}

// TestUploadFile_EPUB_MatchesCatalogBookNotInOwnLibrary: a second user's upload
// attaches to the existing catalog entry instead of duplicating it.
func TestUploadFile_EPUB_MatchesCatalogBookNotInOwnLibrary(t *testing.T) {
	const ownerUser = "catalog-match-owner-user"
	const uploaderUser = "catalog-match-uploader-user"
	app2 := noExternalMatchApp(t, uploaderUser)

	ub := seedBookInLibrary(t, ownerUser, "The Silmarillion", "J.R.R. Tolkien", "")
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id IN ($1, $2)`,
			ownerUser, uploaderUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE user_id IN ($1, $2)`,
			ownerUser, uploaderUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.books WHERE id = $1`, ub.BookID)
	})

	data := buildEPUBBytes("The Silmarillion", "J.R.R. Tolkien", "")
	uploadID, _, _, err := app2.Services.Books.CreateUpload(
		context.Background(), uploaderUser, "silm.epub", "application/epub+zip",
		int64(len(data)), "",
	)
	require.NoError(t, err)
	require.NoError(t, fakeStore.Put(
		context.Background(), uploadID,
		bytes.NewReader(data), int64(len(data)), "application/epub+zip",
	))

	result, err := app2.Services.Books.FinalizeUpload(
		context.Background(), uploaderUser, uploadID,
		"silm.epub", "application/epub+zip", "", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(
		t,
		result.MatchedExisting,
		"existing catalog book must link, not duplicate",
	)
	assert.Equal(t, ub.BookID, result.UserBook.BookID)
	assert.NotEqual(
		t, ub.ID, result.UserBook.ID,
		"the uploader must get their own user_book row, not the owner's",
	)

	attached, err := testApp.Services.Books.GetUserBook(
		context.Background(), uploaderUser, ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, ub.BookID, attached.BookID)
}

// TestUploadFile_EPUB_NormalizedMatch_DifferentAuthor_NoFalsePositive: same
// title, different author must not link.
func TestUploadFile_EPUB_NormalizedMatch_DifferentAuthor_NoFalsePositive(t *testing.T) {
	const isolatedUser = "norm-false-positive-user"
	app2 := noExternalMatchApp(t, isolatedUser)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.user_books WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM books.book_files WHERE user_id = $1`, isolatedUser)
	})

	seedBookInLibrary(t, isolatedUser, "Hamlet", "William Shakespeare", "")

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
		"hamlet.epub", "application/epub+zip", "", "", "",
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, bsvc.ErrUnrecognizedBook,
		"different author must not create a false-positive link")
}

// TestConnectFinalizeBookUpload_Unrecognized_ReturnsInvalidArgument checks the
// error mapping.
func TestConnectFinalizeBookUpload_Unrecognized_ReturnsInvalidArgument(t *testing.T) {
	const isolatedUser = "handler-unrecognized-upload-user"
	app2 := noExternalMatchApp(t, isolatedUser)

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := newBooksClientFor(ts.URL, connect.WithHTTPGet())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := buildEPUBBytes("HandlerUnrecognizedBook", "Unknown Author Handler", "")
	createReq := connect.NewRequest(&booksv1.CreateBookUploadRequest{
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

	finalReq := connect.NewRequest(&booksv1.FinalizeBookUploadRequest{
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
