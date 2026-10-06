package books_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
)

// setupPagedBook gives owner a PDF-only book whose KEPUB is mocks.PagedEPUB
// run through the real kepubify, with Kobo sync on.
func setupPagedBook(t *testing.T, owner string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	bookID := seedBookInLibrary(t, owner, "Paged-"+uuid.NewString(), "Author", "").BookID

	pdfData := minimalPDFData()
	key := fmt.Sprintf("users/%s/books/%s/paged.pdf", owner, bookID)
	require.NoError(t, fakeStore.Put(ctx, key, bytes.NewReader(pdfData),
		int64(len(pdfData)), "application/pdf"))
	_, err := testApp.Repositories.BookFiles.Insert(ctx,
		models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID: bookID, UserID: owner, Format: models.FileFormatPDF,
			StorageKey: key, SizeBytes: int64(len(pdfData)),
			Status: models.FileStatusReady,
		})
	require.NoError(t, err)

	convertPaged(t, owner, bookID, 0)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, owner, bookID))
	return bookID
}

// convertPaged (re)generates bookID's KEPUB from mocks.PagedEPUB with intro
// extra paragraphs.
func convertPaged(t *testing.T, owner string, bookID uuid.UUID, intro int) {
	t.Helper()
	conv := services.NewConversionService(
		testApp.Logger, testApp.Repositories.Books, testApp.Repositories.BookFiles,
		fakeStore, nil, fakePDFConverter(mocks.PagedEPUB("Paged", "Author", intro)),
	)
	kepub, err := conv.EnsureKEPUB(context.Background(), owner, bookID)
	require.NoError(t, err)
	require.Equal(t, models.FileStatusReady, kepub.Status)
}

// regeneratePaged replaces the KEPUB the device has (an older converter
// version) with one whose spans moved.
func regeneratePaged(t *testing.T, owner string, bookID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := testDB.Exec(ctx,
		`UPDATE books.book_files SET converter_version = converter_version - 1
		 WHERE book_id = $1 AND user_id = $2 AND format = 'kepub'`, bookID, owner)
	require.NoError(t, err)
	convertPaged(t, owner, bookID, 1)
	_, err = testDB.Exec(ctx,
		`UPDATE books.user_books SET kobo_last_synced_converter_version = $3
		 WHERE book_id = $1 AND user_id = $2`,
		bookID, owner, services.CurrentKEPUBConverterVersion()-1)
	require.NoError(t, err)
}

func pagedSpan(source, value string) map[string]any {
	return map[string]any{"Source": source, "Type": "KoboSpan", "Value": value}
}

func pagedPut(value string) any {
	return chapterTwoPut(mocks.PagedDocOne, value)
}

func webPage(t *testing.T, owner string, bookID uuid.UUID, page int) {
	t.Helper()
	readAt := time.Now().Add(-time.Second)
	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(),
		models.BookReadingState{ //nolint:exhaustruct //optional fields
			UserID: owner, BookID: bookID, Source: models.ReadingSourceWeb,
			Percent: 50, ReadAt: &readAt,
			Position: &models.ReadingPosition{Href: "", Offset: 0, Page: page},
		},
	))
}

// changedEntitlementState is the ReadingState of the sync's ChangedEntitlement.
func changedEntitlementState(t *testing.T, entries []map[string]any) map[string]any {
	t.Helper()
	for _, e := range entries {
		if ce, ok := e["ChangedEntitlement"].(map[string]any); ok {
			rs, isMap := ce["ReadingState"].(map[string]any)
			require.True(t, isMap)
			return rs
		}
	}
	t.Fatal("no ChangedEntitlement")
	return nil
}

func TestKoboPutState_PDFSourcedSpan_StoresPage(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-span-to-page-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)

	koboPutState(t, ts, rawToken, bookID, pagedPut("kobo.3.1"))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	require.NotNil(t, state.Position)
	assert.Equal(t, models.ReadingPosition{Href: "", Offset: 0, Page: 2},
		*state.Position)
}

func TestKoboLibrarySync_PDFPage_ChangedReadingStateCarriesSpan(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-page-to-span-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)
	koboSync(t, ts, rawToken)

	webPage(t, owner, bookID, 3)
	changed := changedReadingStates(koboSync(t, ts, rawToken))
	require.Len(t, changed, 1)
	assert.Equal(t, pagedSpan(mocks.PagedDocTwo, "kobo.1.1"),
		bookmarkLocation(t, changed[0]), "the first span of page 3")

	webPage(t, owner, bookID, 2)
	assert.Equal(t, pagedSpan(mocks.PagedDocOne, "kobo.3.1"),
		bookmarkLocation(t, koboGetState(t, ts, rawToken, bookID)))
}

func TestKoboSync_PDFSourcedRegeneration_ReDerivesStaleLocation(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-regen-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)
	koboSync(t, ts, rawToken)
	koboPutState(t, ts, rawToken, bookID, pagedPut("kobo.3.1"))

	regeneratePaged(t, owner, bookID)

	rs := changedEntitlementState(t, koboSync(t, ts, rawToken))
	assert.Equal(t, pagedSpan(mocks.PagedDocOne, "kobo.4.1"), bookmarkLocation(t, rs),
		"page 2's span in the new KEPUB, not the old kobo.3.1")
	assert.Equal(t, pagedSpan(mocks.PagedDocOne, "kobo.4.1"),
		bookmarkLocation(t, koboGetState(t, ts, rawToken, bookID)))
	changed := changedReadingStates(koboSync(t, ts, rawToken))
	require.Len(t, changed, 1, "the cleared bookmark goes out again")
	assert.Equal(t, pagedSpan(mocks.PagedDocOne, "kobo.4.1"),
		bookmarkLocation(t, changed[0]))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.ReadingPosition{Href: "", Offset: 0, Page: 2},
		*state.Position, "the stored page survives the regeneration")
}

func TestKoboSync_PDFSourcedRegeneration_NoPositionIsPercentOnly(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-regen-nopos-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)
	koboSync(t, ts, rawToken)
	koboPutState(t, ts, rawToken, bookID, pagedPut("kobo.99.1"))

	regeneratePaged(t, owner, bookID)

	rs := changedEntitlementState(t, koboSync(t, ts, rawToken))
	assert.Nil(t, bookmarkLocation(t, rs), "never a span from the old KEPUB")
	assert.Nil(t, bookmarkLocation(t, koboGetState(t, ts, rawToken, bookID)))
}

func TestKoboPutState_PDFSourcedOutdatedDevice_NoPosition(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-outdated-put-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)
	koboSync(t, ts, rawToken)
	regeneratePaged(t, owner, bookID)

	koboPutState(t, ts, rawToken, bookID, pagedPut("kobo.3.1"))
	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Nil(t, state.Position,
		"a span of the old KEPUB isn't looked up in the new one")

	rs := changedEntitlementState(t, koboSync(t, ts, rawToken))
	assert.Nil(t, bookmarkLocation(t, rs))
}

func TestKoboSync_PDFSourcedCurrentLocationIsKept(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	owner := "kobo-pdf-current-" + uuid.NewString()
	bookID := setupPagedBook(t, owner)
	rawToken := registerTestDevice(t, owner)
	koboPutState(t, ts, rawToken, bookID, pagedPut("kobo.2.1"))

	assert.Equal(t, pagedSpan(mocks.PagedDocOne, "kobo.2.1"),
		bookmarkLocation(t, koboGetState(t, ts, rawToken, bookID)),
		"the device's own bookmark, not page 1's first span")
}
