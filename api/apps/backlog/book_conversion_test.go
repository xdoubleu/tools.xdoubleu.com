package backlog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
)

// fakeEPUBConverter is a test double that returns fixed KEPUB bytes.
type fakeEPUBConverter struct {
	out []byte
	err error
}

func (f *fakeEPUBConverter) Convert(_ context.Context, _ []byte) ([]byte, error) {
	return f.out, f.err
}

// newTestConversionService constructs a ConversionService using the real DB
// repository from testApp, a fresh fake objectstore, and controlled converters.
// Pass nil for convertPDF to use the default (calibrePDFConverter), or supply a
// fake for tests that exercise the PDF path.
func newTestConversionService(
	converter services.EPUBConverter,
	convertPDF services.PDFConverter,
) (*services.ConversionService, *objectstore.FakeClient) {
	store := objectstore.NewFake()
	svc := services.NewConversionService(
		testApp.Logger,
		testApp.Repositories.BookFiles,
		store,
		converter,
		convertPDF,
	)
	return svc, store
}

// fakePDFConverter is a test double for the PDF→EPUB subprocess. It writes
// the provided EPUB bytes to outPath so the rest of the pipeline can proceed.
func fakePDFConverter(epubBytes []byte) services.PDFConverter {
	return func(_ context.Context, _ string, outPath string) error {
		return os.WriteFile(outPath, epubBytes, 0o600)
	}
}

// failingPDFConverter is a test double that always returns an error.
func failingPDFConverter(_ context.Context, _, _ string) error {
	return errors.New("pdf converter: simulated failure")
}

// failingPutStore wraps FakeClient but makes Put always fail.
type failingPutStore struct{ *objectstore.FakeClient }

func (f *failingPutStore) Put(
	_ context.Context, _ string, _ io.Reader, _ int64, _ string,
) error {
	return errors.New("put: simulated failure")
}

// failingGetStore wraps FakeClient but makes Get always fail.
type failingGetStore struct{ *objectstore.FakeClient }

func (f *failingGetStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("get: simulated failure")
}

func (f *failingGetStore) PresignGet(
	_ context.Context, _ string, _ time.Duration,
) (string, error) {
	return "", errors.New("presign: simulated failure")
}

// seedEPUBFile stores a minimal EPUB in the given store and inserts a book_files
// row so EnsureKEPUB has a source to convert.
func seedEPUBFile(
	t *testing.T,
	store *objectstore.FakeClient,
	bookID uuid.UUID,
) *models.BookFile {
	t.Helper()

	epubData := buildEPUBBytes("Seed Book", "Seed Author", "")
	key := fmt.Sprintf("users/%s/books/%s/seed.epub", userID, bookID)

	require.NoError(t,
		store.Put(
			context.Background(),
			key,
			bytes.NewReader(epubData),
			int64(len(epubData)),
			"application/epub+zip",
		),
	)

	bf, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     bookID,
			UserID:     userID,
			Format:     models.FileFormatEPUB,
			StorageKey: key,
			SizeBytes:  int64(len(epubData)),
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)
	return bf
}

// --- EnsureKEPUB tests ---

func TestEnsureKEPUB_ConvertSuccess(t *testing.T) {
	book := addUniqueBook(t)
	kepubBytes := []byte("fake kepub content")
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: kepubBytes, err: nil},
		nil,
	)
	seedEPUBFile(t, store, book.ID)

	result, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, models.FileFormatKEPUB, result.Format)
	assert.Equal(t, models.FileStatusReady, result.Status)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, book.ID, result.BookID)
	assert.NotEmpty(t, result.StorageKey)
	assert.Equal(t, int64(len(kepubBytes)), result.SizeBytes)
	assert.NotNil(t, result.SourceFileID)

	// Verify the KEPUB bytes were actually stored.
	stored, ok := store.GetContent(result.StorageKey)
	assert.True(t, ok, "kepub file should be in objectstore")
	assert.Equal(t, kepubBytes, stored)
}

func TestEnsureKEPUB_Idempotent(t *testing.T) {
	book := addUniqueBook(t)
	kepubBytes := []byte("idempotent kepub")
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: kepubBytes, err: nil},
		nil,
	)
	seedEPUBFile(t, store, book.ID)

	first, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)

	second, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)

	// Second call returns the same row without re-converting.
	assert.Equal(t, first.ID, second.ID)
}

// seedPDFFile stores a minimal PDF in the given store and inserts a book_files
// row so EnsureKEPUB has a PDF source to convert.
func seedPDFFile(
	t *testing.T,
	store *objectstore.FakeClient,
	bookID uuid.UUID,
) *models.BookFile {
	t.Helper()

	pdfData := minimalPDFData()
	key := fmt.Sprintf("users/%s/books/%s/seed.pdf", userID, bookID)

	require.NoError(t,
		store.Put(
			context.Background(),
			key,
			bytes.NewReader(pdfData),
			int64(len(pdfData)),
			"application/pdf",
		),
	)

	bf, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     bookID,
			UserID:     userID,
			Format:     models.FileFormatPDF,
			StorageKey: key,
			SizeBytes:  int64(len(pdfData)),
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)
	return bf
}

func TestEnsureKEPUB_PDFOnly_ConvertSuccess(t *testing.T) {
	book := addUniqueBook(t)
	kepubBytes := []byte("pdf-sourced kepub")
	// fakePDFConverter writes a minimal EPUB so kepubify gets valid input.
	epubBytes := buildEPUBBytes("PDF Book", "PDF Author", "")
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: kepubBytes, err: nil},
		fakePDFConverter(epubBytes),
	)
	seedPDFFile(t, store, book.ID)

	result, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, models.FileFormatKEPUB, result.Format)
	assert.Equal(t, models.FileStatusReady, result.Status)
	assert.Equal(t, int64(len(kepubBytes)), result.SizeBytes)

	stored, ok := store.GetContent(result.StorageKey)
	assert.True(t, ok, "kepub file should be in objectstore")
	assert.Equal(t, kepubBytes, stored)
}

func TestEnsureKEPUB_PDFConvertError_MarksFailedStatus(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("kepub"), err: nil},
		failingPDFConverter,
	)
	seedPDFFile(t, store, book.ID)

	_, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prepare epub source")

	files, listErr := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, listErr)

	var kepubRow *models.BookFile
	for i := range files {
		if files[i].Format == models.FileFormatKEPUB {
			kepubRow = &files[i]
		}
	}
	require.NotNil(t, kepubRow, "kepub row should exist even after failure")
	assert.Equal(t, models.FileStatusFailed, kepubRow.Status)
}

func TestEnsureKEPUB_NoFiles_FailedPrecondition(t *testing.T) {
	book := addUniqueBook(t)
	conv, _ := newTestConversionService(
		&fakeEPUBConverter{out: []byte("kepub"), err: nil},
		nil,
	)

	_, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.Error(t, err)

	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeFailedPrecondition, connectErr.Code())
}

func TestEnsureKEPUB_ConvertError_MarksFailedStatus(t *testing.T) {
	book := addUniqueBook(t)
	convertErr := errors.New("kepubify exploded")
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: nil, err: convertErr},
		nil,
	)
	seedEPUBFile(t, store, book.ID)

	_, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert epub to kepub")

	// The book_files row inserted during conversion must be marked failed.
	files, listErr := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, listErr)

	var kepubRow *models.BookFile
	for i := range files {
		if files[i].Format == models.FileFormatKEPUB {
			kepubRow = &files[i]
		}
	}
	require.NotNil(t, kepubRow, "kepub row should exist even after failure")
	assert.Equal(t, models.FileStatusFailed, kepubRow.Status)
}

// TestBooksFilesRepo_UpdateAfterConversion verifies the new repository method.
func TestBooksFilesRepo_UpdateAfterConversion(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatKEPUB,
		StorageKey: "",
		SizeBytes:  0,
		Status:     models.FileStatusConverting,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	const newKey = "users/test/books/kepub/done.kepub"
	const newSize = int64(9999)

	err = testApp.Repositories.BookFiles.UpdateAfterConversion(
		context.Background(), inserted.ID, newKey, newSize,
	)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		inserted.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, newKey, got.StorageKey)
	assert.Equal(t, newSize, got.SizeBytes)
	assert.Equal(t, models.FileStatusReady, got.Status)
}

// user2ID is a second user for cross-user deduplication tests.
//
//nolint:gochecknoglobals //mirrors the pattern of userID in app_test.go
var user2ID = "5001e9cf-3fbe-4b09-863f-bd1654cfbf76"

// countingConverter wraps a fakeEPUBConverter and records how many times
// Convert is called. Used to assert that cross-user dedup skips conversion.
type countingConverter struct {
	calls int
	out   []byte
	err   error
}

func (c *countingConverter) Convert(_ context.Context, _ []byte) ([]byte, error) {
	c.calls++
	return c.out, c.err
}

// seedEPUBFileForUser stores a canonical EPUB blob in the given store and
// inserts a book_files row for the specified user with the given checksum.
// The storage key is the content-addressed canonical path (books/<checksum>.epub)
// so that both source and derived canonical key follow the same scheme.
func seedEPUBFileForUser(
	t *testing.T,
	store *objectstore.FakeClient,
	bookID uuid.UUID,
	uid string,
	checksum string,
) *models.BookFile {
	t.Helper()

	epubData := buildEPUBBytes("Dedup Book", "Dedup Author", "")
	key := "books/" + checksum + ".epub"

	// Put the blob only once — both users share the same canonical object.
	if _, exists := store.GetContent(key); !exists {
		require.NoError(t, store.Put(
			context.Background(), key,
			bytes.NewReader(epubData), int64(len(epubData)),
			"application/epub+zip",
		))
	}

	bf, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     bookID,
			UserID:     uid,
			Format:     models.FileFormatEPUB,
			StorageKey: key,
			SizeBytes:  int64(len(epubData)),
			Status:     models.FileStatusReady,
			Checksum:   &checksum,
		},
	)
	require.NoError(t, err)
	return bf
}

// TestEnsureKEPUB_CanonicalKey_WhenSourceHasChecksum verifies that when the
// source file has a checksum, the KEPUB is stored at the canonical per-book key
// books/<bookID>/<checksum>.kepub (not a per-user path).
func TestEnsureKEPUB_CanonicalKey_WhenSourceHasChecksum(t *testing.T) {
	book := addUniqueBook(t)
	// Use the book ID as a unique-per-run checksum so parallel test runs do not
	// share the same canonical key across different books.
	checksum := book.ID.String()
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("canonical kepub"), err: nil},
		nil,
	)
	seedEPUBFileForUser(t, store, book.ID, userID, checksum)

	result, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(
		t,
		"books/"+book.ID.String()+"/"+checksum+".kepub",
		result.StorageKey,
		"KEPUB must be stored at per-book canonical key, not a per-user path",
	)
	assert.Equal(t, models.FileStatusReady, result.Status)
}

// TestEnsureKEPUB_CrossUserDedup_SkipsConversion verifies that when a second
// user requests a KEPUB for a book whose source has the same checksum as an
// already-converted canonical blob, no conversion is performed; the second user
// gets a new row pointing at the same canonical storage key.
func TestEnsureKEPUB_CrossUserDedup_SkipsConversion(t *testing.T) {
	book := addUniqueBook(t)
	// Use the book ID as a unique-per-run checksum so parallel test runs do not
	// share the same canonical key across different books.
	checksum := book.ID.String()
	counter := &countingConverter{
		calls: 0,
		out:   []byte("dedup kepub content"),
		err:   nil,
	}
	conv, store := newTestConversionService(counter, nil)

	// Both users own the same source EPUB (shared canonical blob).
	seedEPUBFileForUser(t, store, book.ID, userID, checksum)
	seedEPUBFileForUser(t, store, book.ID, user2ID, checksum)

	// User 1: cold path — conversion runs.
	result1, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.calls, "converter must run exactly once for user 1")
	assert.Equal(t, "books/"+book.ID.String()+"/"+checksum+".kepub", result1.StorageKey)

	// User 2: warm path — canonical blob already exists; converter must NOT run.
	result2, err := conv.EnsureKEPUB(context.Background(), user2ID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.calls, "converter must not run again for user 2 (dedup)")
	assert.Equal(t, result1.StorageKey, result2.StorageKey,
		"both users must reference the same canonical blob")
	assert.NotEqual(t, result1.ID, result2.ID,
		"each user gets their own book_files row")
	assert.Equal(t, models.FileStatusReady, result2.Status)
}

func TestEnsureKEPUB_StorePutFails_MarksFailedStatus(t *testing.T) {
	book := addUniqueBook(t)
	inner := objectstore.NewFake()
	store := &failingPutStore{FakeClient: inner}

	// Seed the EPUB into the inner fake so Get succeeds but Put fails.
	epubData := buildEPUBBytes("PutFail Book", "PutFail Author", "")
	key := fmt.Sprintf("users/%s/books/%s/seed.epub", userID, book.ID)
	require.NoError(
		t,
		inner.Put(
			context.Background(),
			key,
			bytes.NewReader(epubData),
			int64(len(epubData)),
			"application/epub+zip",
		),
	)
	_, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     book.ID,
			UserID:     userID,
			Format:     models.FileFormatEPUB,
			StorageKey: key,
			SizeBytes:  int64(len(epubData)),
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)

	conv := services.NewConversionService(
		testApp.Logger,
		testApp.Repositories.BookFiles,
		store,
		&fakeEPUBConverter{out: []byte("kepub"), err: nil},
		nil,
	)

	_, err = conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store kepub")

	files, listErr := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, listErr)
	var kepubRow *models.BookFile
	for i := range files {
		if files[i].Format == models.FileFormatKEPUB {
			kepubRow = &files[i]
		}
	}
	require.NotNil(t, kepubRow)
	assert.Equal(t, models.FileStatusFailed, kepubRow.Status)
}

func TestEnsureKEPUB_StoreGetFails_MarksFailedStatus(t *testing.T) {
	book := addUniqueBook(t)
	inner := objectstore.NewFake()
	store := &failingGetStore{FakeClient: inner}

	epubData := buildEPUBBytes("GetFail Book", "GetFail Author", "")
	key := fmt.Sprintf("users/%s/books/%s/seed.epub", userID, book.ID)
	require.NoError(
		t,
		inner.Put(
			context.Background(),
			key,
			bytes.NewReader(epubData),
			int64(len(epubData)),
			"application/epub+zip",
		),
	)
	_, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     book.ID,
			UserID:     userID,
			Format:     models.FileFormatEPUB,
			StorageKey: key,
			SizeBytes:  int64(len(epubData)),
			Status:     models.FileStatusReady,
		},
	)
	require.NoError(t, err)

	conv := services.NewConversionService(
		testApp.Logger,
		testApp.Repositories.BookFiles,
		store,
		&fakeEPUBConverter{out: []byte("kepub"), err: nil},
		nil,
	)

	_, err = conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.Error(t, err)
	// Get failure surfaces as "prepare epub source: download epub: ..."
	assert.Contains(t, err.Error(), "prepare epub source")

	files, listErr := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, listErr)
	var kepubRow *models.BookFile
	for i := range files {
		if files[i].Format == models.FileFormatKEPUB {
			kepubRow = &files[i]
		}
	}
	require.NotNil(t, kepubRow)
	assert.Equal(t, models.FileStatusFailed, kepubRow.Status)
}
