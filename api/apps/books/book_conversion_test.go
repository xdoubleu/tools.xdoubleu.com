package books_test

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

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database"
)

type fakeEPUBConverter struct {
	out []byte
	err error
}

func (f *fakeEPUBConverter) Convert(_ context.Context, _ []byte) ([]byte, error) {
	return f.out, f.err
}

// newTestConversionService uses testApp's DB, a fresh fake store, and the
// given converters (nil convertPDF selects goPDFConverter).
func newTestConversionService(
	converter services.EPUBConverter,
	convertPDF services.PDFConverter,
) (*services.ConversionService, *objectstore.FakeClient) {
	store := objectstore.NewFake()
	svc := services.NewConversionService(
		testApp.Logger,
		testApp.Repositories.Books,
		testApp.Repositories.BookFiles,
		store,
		converter,
		convertPDF,
	)
	return svc, store
}

// fakePDFConverter writes epubBytes to outPath.
func fakePDFConverter(epubBytes []byte) services.PDFConverter {
	return func(
		_ context.Context, _, outPath, _, _ string, _ []string,
	) error {
		return os.WriteFile(outPath, epubBytes, 0o600)
	}
}

func failingPDFConverter(
	_ context.Context, _, _, _, _ string, _ []string,
) error {
	return errors.New("pdf converter: simulated failure")
}

type failingPutStore struct{ *objectstore.FakeClient }

func (f *failingPutStore) Put(
	_ context.Context, _ string, _ io.Reader, _ int64, _ string,
) error {
	return errors.New("put: simulated failure")
}

type failingGetStore struct{ *objectstore.FakeClient }

func (f *failingGetStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("get: simulated failure")
}

func (f *failingGetStore) PresignGet(
	_ context.Context, _ string, _ time.Duration,
) (string, error) {
	return "", errors.New("presign: simulated failure")
}

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

	assert.Equal(t, first.ID, second.ID)
}

// TestEnsureKEPUB_StaleConverterVersion_Regenerates: a row from an older
// converter (0 is the migration default) is regenerated.
func TestEnsureKEPUB_StaleConverterVersion_Regenerates(t *testing.T) {
	book := addUniqueBook(t)
	conv, store := newTestConversionService(
		&fakeEPUBConverter{out: []byte("fresh kepub content"), err: nil},
		nil,
	)
	source := seedEPUBFile(t, store, book.ID)

	sourceID := source.ID
	staleKey := fmt.Sprintf("users/%s/books/%s/stale.kepub", userID, book.ID)
	require.NoError(t, store.Put(
		context.Background(), staleKey,
		bytes.NewReader([]byte("stale kepub content")), 20, "application/epub+zip",
	))
	stale, err := testApp.Repositories.BookFiles.Insert(
		context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:       book.ID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   staleKey,
			SizeBytes:    20,
			Status:       models.FileStatusReady,
			SourceFileID: &sourceID,
		},
	)
	require.NoError(t, err)
	require.Equal(t, int16(0), stale.ConverterVersion)

	result, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)

	assert.NotEqual(t, stale.ID, result.ID, "stale row should be replaced, not reused")
	assert.Positive(t, result.ConverterVersion)

	stored, ok := store.GetContent(result.StorageKey)
	assert.True(t, ok, "regenerated kepub should be in objectstore")
	assert.Equal(t, []byte("fresh kepub content"), stored)

	_, getErr := testApp.Repositories.BookFiles.GetByID(context.Background(), stale.ID)
	assert.ErrorIs(
		t,
		getErr,
		database.ErrResourceNotFound,
		"stale row should be deleted",
	)
}

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
	const newVersion = int16(1)

	err = testApp.Repositories.BookFiles.UpdateAfterConversion(
		context.Background(), inserted.ID, newKey, newSize, newVersion,
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
	assert.Equal(t, newVersion, got.ConverterVersion)
}

// user2ID is a second user for cross-user deduplication tests.
//
//nolint:gochecknoglobals //mirrors the pattern of userID in app_test.go
var user2ID = "5001e9cf-3fbe-4b09-863f-bd1654cfbf76"

// countingConverter counts Convert calls.
type countingConverter struct {
	calls int
	out   []byte
	err   error
}

func (c *countingConverter) Convert(_ context.Context, _ []byte) ([]byte, error) {
	c.calls++
	return c.out, c.err
}

// seedEPUBFileForUser stores a canonical EPUB blob and a book_files row for
// the user with the given checksum.
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

	// Both users share the canonical object.
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

// TestEnsureKEPUB_CanonicalKey_WhenSourceHasChecksum: the KEPUB lands at
// books/<bookID>/<checksum>.kepub.
func TestEnsureKEPUB_CanonicalKey_WhenSourceHasChecksum(t *testing.T) {
	book := addUniqueBook(t)
	// Unique per run so runs don't share a canonical key.
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

// TestEnsureKEPUB_CrossUserDedup_SkipsConversion: a second user with the same
// source checksum reuses the blob without converting.
func TestEnsureKEPUB_CrossUserDedup_SkipsConversion(t *testing.T) {
	book := addUniqueBook(t)
	// Unique per run so runs don't share a canonical key.
	checksum := book.ID.String()
	counter := &countingConverter{
		calls: 0,
		out:   []byte("dedup kepub content"),
		err:   nil,
	}
	conv, store := newTestConversionService(counter, nil)

	seedEPUBFileForUser(t, store, book.ID, userID, checksum)
	seedEPUBFileForUser(t, store, book.ID, user2ID, checksum)

	result1, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.calls, "converter must run exactly once for user 1")
	assert.Equal(t, "books/"+book.ID.String()+"/"+checksum+".kepub", result1.StorageKey)

	result2, err := conv.EnsureKEPUB(context.Background(), user2ID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.calls, "converter must not run again for user 2 (dedup)")
	assert.Equal(t, result1.StorageKey, result2.StorageKey,
		"both users must reference the same canonical blob")
	assert.NotEqual(t, result1.ID, result2.ID,
		"each user gets their own book_files row")
	assert.Equal(t, models.FileStatusReady, result2.Status)
}

// TestEnsureKEPUB_StaleCanonicalBlob_ReconvertsForNewUser: a stale shared blob
// is reconverted, not handed to the second user.
func TestEnsureKEPUB_StaleCanonicalBlob_ReconvertsForNewUser(t *testing.T) {
	book := addUniqueBook(t)
	checksum := book.ID.String()
	counter := &countingConverter{
		calls: 0,
		out:   []byte("refreshed canonical kepub"),
		err:   nil,
	}
	conv, store := newTestConversionService(counter, nil)

	seedEPUBFileForUser(t, store, book.ID, userID, checksum)
	seedEPUBFileForUser(t, store, book.ID, user2ID, checksum)

	canonical, err := conv.EnsureKEPUB(context.Background(), userID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.calls)

	require.NoError(t, testApp.Repositories.BookFiles.UpdateAfterConversion(
		context.Background(),
		canonical.ID,
		canonical.StorageKey,
		canonical.SizeBytes,
		0,
	))

	result2, err := conv.EnsureKEPUB(context.Background(), user2ID, book.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, counter.calls, "converter must run again for the stale blob")

	stored, ok := store.GetContent(result2.StorageKey)
	assert.True(t, ok)
	assert.Equal(t, []byte("refreshed canonical kepub"), stored)
	assert.Positive(t, result2.ConverterVersion)
}

func TestEnsureKEPUB_StorePutFails_MarksFailedStatus(t *testing.T) {
	book := addUniqueBook(t)
	inner := objectstore.NewFake()
	store := &failingPutStore{FakeClient: inner}

	// Get succeeds but Put fails.
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
		testApp.Repositories.Books,
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
		testApp.Repositories.Books,
		testApp.Repositories.BookFiles,
		store,
		&fakeEPUBConverter{out: []byte("kepub"), err: nil},
		nil,
	)

	_, err = conv.EnsureKEPUB(context.Background(), userID, book.ID)
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
	require.NotNil(t, kepubRow)
	assert.Equal(t, models.FileStatusFailed, kepubRow.Status)
}
