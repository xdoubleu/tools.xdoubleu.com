package books_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services/conversion"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/logging"
)

// figureAnalysisLimit drains every pending file other tests left behind.
const figureAnalysisLimit = 1000

// addFigureFile stores data under a fresh key and adds a ready file row for
// the user's book.
func addFigureFile(
	t *testing.T, user string, bookID uuid.UUID, format string, data []byte,
) string {
	t.Helper()
	key := "books/" + bookID.String() + "/" + uuid.NewString() + "." + format
	if data != nil {
		require.NoError(t, fakeStore.Put(
			context.Background(), key, bytes.NewReader(data), int64(len(data)), "",
		))
	}
	insertFigureRow(t, user, bookID, format, key)
	return key
}

func insertFigureRow(t *testing.T, user string, bookID uuid.UUID, format, key string) {
	t.Helper()
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(),
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     bookID,
			UserID:     user,
			Format:     format,
			StorageKey: key,
			SizeBytes:  1,
			Status:     models.FileStatusReady,
		})
	require.NoError(t, err)
}

func librarySuggestion(t *testing.T, bookID uuid.UUID) *booksv1.ReadSuggestion {
	t.Helper()
	lib, _, err := testApp.BuildSharedLibrary(context.Background(), userID)
	require.NoError(t, err)
	for _, ub := range lib.GetWishlist() {
		if ub.GetBookId() == bookID.String() {
			return ub.GetReadSuggestion()
		}
	}
	require.Fail(t, "book not in wishlist")
	return nil
}

func TestFigureAnalysis_SuggestsByDensity(t *testing.T) {
	ctx := context.Background()
	heavy := addTestBookNoISBN(t, "Figure-heavy "+uuid.NewString())
	prose := addTestBookNoISBN(t, "Prose "+uuid.NewString())
	epub := models.FileFormatEPUB
	addFigureFile(t, userID, heavy.BookID, epub, mocks.FigureEPUB(3, 4000))
	addFigureFile(t, userID, prose.BookID, epub, mocks.FigureEPUB(0, 9000))

	assert.Nil(t, librarySuggestion(t, heavy.BookID), "unanalysed")

	_, err := testApp.Services.Figures.AnalysePending(ctx, figureAnalysisLimit)
	require.NoError(t, err)

	got := librarySuggestion(t, heavy.BookID)
	require.NotNil(t, got)
	assert.Equal(t, models.FileFormatPDF, got.GetFormat())
	assert.EqualValues(t, 3, got.GetFigureCount())
	assert.EqualValues(t, 2, got.GetPages())

	got = librarySuggestion(t, prose.BookID)
	require.NotNil(t, got)
	assert.Equal(t, models.FileFormatEPUB, got.GetFormat())
	assert.EqualValues(t, 5, got.GetPages())
}

// A deduplicated upload inherits its blob's analysis; an unparsable file and
// a missing blob are recorded as unanalysable, so they aren't retried.
func TestFigureAnalysis_CopyFailAndRetry(t *testing.T) {
	ctx := context.Background()
	book := addTestBookNoISBN(t, "Shared "+uuid.NewString())
	broken := addTestBookNoISBN(t, "Broken "+uuid.NewString())
	missing := addTestBookNoISBN(t, "Missing "+uuid.NewString())
	key := addFigureFile(
		t, userID, book.BookID, models.FileFormatEPUB, mocks.FigureEPUB(1, 100),
	)
	addFigureFile(t, userID, broken.BookID, models.FileFormatPDF, []byte("not a pdf"))
	addFigureFile(t, userID, missing.BookID, models.FileFormatEPUB, nil)

	_, err := testApp.Services.Figures.AnalysePending(ctx, figureAnalysisLimit)
	require.NoError(t, err)

	other := "figure-user-" + uuid.NewString()
	insertFigureRow(t, other, book.BookID, models.FileFormatEPUB, key)
	_, err = testApp.Services.Figures.AnalysePending(ctx, figureAnalysisLimit)
	require.NoError(t, err)

	files, err := testApp.Services.Books.FilesByUser(ctx, other)
	require.NoError(t, err)
	assert.Equal(t,
		models.FigureStats{Figures: 1, Pages: 1},
		files[book.BookID].Figures[models.FileFormatEPUB],
	)

	files, err = testApp.Services.Books.FilesByUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t,
		models.FigureStats{Figures: 0, Pages: 0},
		files[broken.BookID].Figures[models.FileFormatPDF],
	)
	assert.Nil(t, librarySuggestion(t, broken.BookID))
	assert.Equal(t,
		models.FigureStats{Figures: 0, Pages: 0},
		files[missing.BookID].Figures[models.FileFormatEPUB],
	)

	pending, err := testApp.Repositories.BookFiles.ListUnanalysed(ctx, figureAnalysisLimit)
	require.NoError(t, err)
	keys := make([]string, 0, len(pending))
	for _, p := range pending {
		keys = append(keys, p.StorageKey)
	}
	assert.NotContains(t, keys, key)
}

func TestFigureAnalysis_Cancelled(t *testing.T) {
	book := addTestBookNoISBN(t, "Cancelled "+uuid.NewString())
	addFigureFile(t, userID, book.BookID, models.FileFormatEPUB, mocks.FigureEPUB(1, 100))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testApp.Services.Figures.AnalysePending(ctx, figureAnalysisLimit)
	require.ErrorIs(t, err, context.Canceled)
}

// getFailingStore fails every Get while the blob still exists.
type getFailingStore struct{ *objectstore.FakeClient }

func (getFailingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("connection reset")
}

func TestFigureAnalysis_FetchFailureRetried(t *testing.T) {
	ctx := context.Background()
	book := addTestBookNoISBN(t, "Flaky "+uuid.NewString())
	key := addFigureFile(
		t, userID, book.BookID, models.FileFormatEPUB, mocks.FigureEPUB(1, 100),
	)

	figures := conversion.NewFigureService(
		logging.NewNopLogger(),
		testApp.Repositories.BookFiles,
		getFailingStore{fakeStore},
	)
	_, err := figures.AnalysePending(ctx, figureAnalysisLimit)
	require.NoError(t, err)

	pending, err := testApp.Repositories.BookFiles.ListUnanalysed(ctx, figureAnalysisLimit)
	require.NoError(t, err)
	keys := make([]string, 0, len(pending))
	for _, p := range pending {
		keys = append(keys, p.StorageKey)
	}
	assert.Contains(t, keys, key)
}
