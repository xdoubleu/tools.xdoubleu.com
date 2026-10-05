package books_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
)

// secondSentence is where kobo.1.2 ("It continues.") starts in chapter two.
//
//nolint:gochecknoglobals // derived fixture constant
var secondSentence = strings.Index(mocks.ChapterTwoText, "It continues.")

// setupKoboChapterBook is setupKoboSyncBook over mocks.ChapterEPUB, converted
// by the real kepubify.
func setupKoboChapterBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	title := "Chapters-" + uuid.NewString()
	seedBookInLibrary(t, ownerID, title, "Author", "")
	result, err := uploadViaTestApp(t, ownerID, "chapters.epub",
		mocks.ChapterEPUB(title, "Author"))
	require.NoError(t, err)
	bookID := result.UserBook.BookID
	_, err = testApp.Services.Conversion.EnsureKEPUB(
		context.Background(), ownerID, bookID,
	)
	require.NoError(t, err)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), ownerID, bookID,
	))
	return registerTestDevice(t, ownerID), bookID
}

func chapterTwoPut(source, value string) any {
	return map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{
				"LastModified":    "2026-09-30T08:15:42Z",
				"ProgressPercent": 40,
				"Location": map[string]any{
					"Source": source, "Type": "KoboSpan", "Value": value,
				},
			},
			"StatusInfo": map[string]any{"Status": "Reading"},
		}},
	}
}

func webWrite(t *testing.T, owner string, bookID uuid.UUID, offset int) {
	t.Helper()
	readAt := time.Now().Add(-time.Second)
	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(),
		models.BookReadingState{ //nolint:exhaustruct //optional fields
			UserID: owner, BookID: bookID, Source: models.ReadingSourceWeb,
			Percent: 45, ReadAt: &readAt,
			Position: &models.ReadingPosition{
				Href: mocks.ChapterTwoHref, Offset: offset, Page: 0,
			},
		},
	))
}

func bookmarkLocation(t *testing.T, rs map[string]any) any {
	t.Helper()
	bm, ok := rs["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	return bm["Location"]
}

func wantSpan(value string) map[string]any {
	return map[string]any{
		"Source": mocks.ChapterTwoHref, "Type": "KoboSpan", "Value": value,
	}
}

func TestKoboPutState_KoboSpan_StoresNeutralPosition(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	for _, source := range []string{mocks.ChapterTwoHref, "Text/ch2.xhtml"} {
		owner := "kobo-span-to-neutral-" + uuid.NewString()
		rawToken, bookID := setupKoboChapterBook(t, owner)

		koboPutState(t, ts, rawToken, bookID, chapterTwoPut(source, "kobo.1.2"))

		state, err := testApp.Services.Books.GetReadingState(
			context.Background(), owner, bookID,
		)
		require.NoError(t, err)
		require.NotNil(t, state.Position, source)
		assert.Equal(t, models.ReadingPosition{
			Href: mocks.ChapterTwoHref, Offset: secondSentence, Page: 0,
		}, *state.Position)
		require.NotNil(t, state.KoboLocation)
		assert.Equal(t, source, state.KoboLocation.Source,
			"the device's own form is kept")
	}
}

func TestKoboPutState_UnknownSpan_NoPosition(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-unknown-span-" + uuid.NewString()
	rawToken, bookID := setupKoboChapterBook(t, owner)
	koboPutState(t, ts, rawToken, bookID,
		chapterTwoPut(mocks.ChapterTwoHref, "kobo.99.9"))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Nil(t, state.Position)
	require.NotNil(t, state.KoboLocation)
	assert.Equal(t, "kobo.99.9", state.KoboLocation.Value)
}

func TestKoboLibrarySync_WebPosition_ChangedReadingStateCarriesSpan(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-neutral-to-span-" + uuid.NewString()
	rawToken, bookID := setupKoboChapterBook(t, owner)
	koboSync(t, ts, rawToken)

	webWrite(t, owner, bookID, secondSentence+5)

	changed := changedReadingStates(koboSync(t, ts, rawToken))
	require.Len(t, changed, 1)
	assert.Equal(t, wantSpan("kobo.1.2"), bookmarkLocation(t, changed[0]))
	assert.Empty(t, changedReadingStates(koboSync(t, ts, rawToken)))
}

func TestKoboLibrarySync_WebPositionBeforeFirstSync_NewEntitlementCarriesSpan(
	t *testing.T,
) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-neutral-new-entitlement-" + uuid.NewString()
	rawToken, bookID := setupKoboChapterBook(t, owner)
	webWrite(t, owner, bookID, 0)

	entries := koboSync(t, ts, rawToken)
	require.Len(t, entries, 1)
	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok)
	rs, ok := ne["ReadingState"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, wantSpan("kobo.1.1"), bookmarkLocation(t, rs))
}

func TestKoboGetState_WebPosition_CarriesSpan(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-neutral-get-state-" + uuid.NewString()
	rawToken, bookID := setupKoboChapterBook(t, owner)
	webWrite(t, owner, bookID, len(mocks.ChapterTwoText)-1)

	rs := koboGetState(t, ts, rawToken, bookID)
	assert.Equal(t, wantSpan("kobo.2.1"), bookmarkLocation(t, rs))
}

func TestKoboLibrarySync_WebPosition_PDFFormatBook_PercentOnly(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-neutral-pdf-format-" + uuid.NewString()
	rawToken, bookID := setupKoboChapterBook(t, owner)
	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		context.Background(), owner, bookID,
		[]string{models.TagKoboSync, models.TagKoboFormatPDF}, true,
	))
	webWrite(t, owner, bookID, secondSentence)

	assert.Nil(t, bookmarkLocation(t, koboGetState(t, ts, rawToken, bookID)),
		"a device reading the PDF gets no KEPUB span")
}

// TestSyncKoboDeviceReadingStates_PendingIsRetried: a state whose span map is
// still building is neither sent nor marked held, so the next sync sends it.
func TestSyncKoboDeviceReadingStates_PendingIsRetried(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-pending-" + uuid.NewString()
	_, onDevice := setupKoboChapterBook(t, owner)
	_, newBook := setupKoboChapterBook(t, owner)
	device, _, err := testApp.Services.Kobo.RegisterKoboDevice(ctx, owner, "Kobo", "")
	require.NoError(t, err)
	deviceID := device.ID
	svc := testApp.Services.Books
	ids := []uuid.UUID{onDevice}

	states, err := svc.ListReadingStates(ctx, owner)
	require.NoError(t, err)
	_, err = svc.SyncKoboDeviceReadingStates(ctx, deviceID, ids, states, nil)
	require.NoError(t, err)

	webWrite(t, owner, onDevice, 0)
	webWrite(t, owner, newBook, 0)
	states, err = svc.ListReadingStates(ctx, owner)
	require.NoError(t, err)
	ids = []uuid.UUID{onDevice, newBook}
	pending := map[uuid.UUID]bool{onDevice: true, newBook: true}

	changed, err := svc.SyncKoboDeviceReadingStates(ctx, deviceID, ids, states, pending)
	require.NoError(t, err)
	assert.Empty(t, changed)

	changed, err = svc.SyncKoboDeviceReadingStates(ctx, deviceID, ids, states, nil)
	require.NoError(t, err)
	got := make([]uuid.UUID, len(changed))
	for i, s := range changed {
		got[i] = s.BookID
	}
	assert.ElementsMatch(t, []uuid.UUID{onDevice, newBook}, got,
		"the on-device book is retried; the new book now gets a ChangedReadingState")

	changed, err = svc.SyncKoboDeviceReadingStates(ctx, deviceID, ids, states, nil)
	require.NoError(t, err)
	assert.Empty(t, changed)
}
