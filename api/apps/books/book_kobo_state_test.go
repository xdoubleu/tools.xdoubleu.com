package books_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func TestKoboPutState_ReturnsUpdateResultsAck(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, bookID := setupKoboSyncBook(t, "kobo-ack-"+uuid.NewString())

	ack := koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "2026-09-30T08:15:42.123Z"))

	want := map[string]any{
		"RequestResult": "Success",
		"UpdateResults": []any{map[string]any{
			"EntitlementId":         bookID.String(),
			"CurrentBookmarkResult": map[string]any{"Result": "Success"},
			"StatisticsResult":      map[string]any{"Result": "Ignored"},
			"StatusInfoResult":      map[string]any{"Result": "Success"},
		}},
	}
	assert.Equal(t, want, ack)
}

func TestKoboPutState_EmptyReadingStates_AcksNothing(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, bookID := setupKoboSyncBook(t, "kobo-ack-empty-"+uuid.NewString())

	ack := koboPutState(t, ts, rawToken, bookID,
		map[string]any{"ReadingStates": []any{}})

	assert.Equal(t, "Success", ack["RequestResult"])
	assert.Equal(t, []any{}, ack["UpdateResults"])
}

func TestKoboPutState_StoresFullLocationAndReadAt(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-full-location-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "2026-09-30T08:15:42.123Z"))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	require.NotNil(t, state.KoboLocation)
	assert.Equal(t, models.KoboLocation{
		Source: "OEBPS/chapter03.xhtml",
		Type:   "KoboSpan",
		Value:  "kobo.12.3",
	}, *state.KoboLocation)
	require.NotNil(t, state.Location)
	assert.Equal(t, "kobo.12.3", *state.Location,
		"location keeps holding the bare span value")
	require.NotNil(t, state.ReadAt)
	assert.True(t, time.Date(2026, 9, 30, 8, 15, 42, 123000000, time.UTC).
		Equal(*state.ReadAt), "read_at is the device bookmark's LastModified")
}

// TestKoboPutState_FinishedDropsKoboLocation: a Finished PUT reports the first
// resource as Source, so the location is not trusted.
func TestKoboPutState_FinishedDropsKoboLocation(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-finished-location-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(100, "kobo.1.1", "Finished", "2026-09-30T08:15:42Z"))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Nil(t, state.KoboLocation)
	assert.Equal(t, 100, state.Percent)
}

// TestKoboPutState_BareStringLocation_ValueOnly: a bare-string Location has no
// Source to resume from, so only the span value is kept.
func TestKoboPutState_BareStringLocation_ValueOnly(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-bare-location-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	koboPutState(t, ts, rawToken, bookID, map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{
				"ProgressPercent": 15,
				"Location":        "kobo.4.2",
			},
		}},
	})

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Nil(t, state.KoboLocation)
	require.NotNil(t, state.Location)
	assert.Equal(t, "kobo.4.2", *state.Location)
}

func TestKoboPutState_UnparseableLastModified_NoReadAt(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-bad-lastmodified-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "yesterday"))

	state, err := testApp.Services.Books.GetReadingState(
		context.Background(), owner, bookID,
	)
	require.NoError(t, err)
	assert.Nil(t, state.ReadAt)
	require.NotNil(t, state.KoboLocation)
}

func TestKoboGetState_EmitsLocationObjectAndPriorityTimestamp(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, bookID := setupKoboSyncBook(t, "kobo-get-location-"+uuid.NewString())

	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "2026-09-30T08:15:42Z"))

	state := koboGetState(t, ts, rawToken, bookID)
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"Source": "OEBPS/chapter03.xhtml",
		"Type":   "KoboSpan",
		"Value":  "kobo.12.3",
	}, bm["Location"])

	lastModified, ok := state["LastModified"].(string)
	require.True(t, ok)
	assert.NotEqual(t, "1970-01-01T00:00:00Z", lastModified)
	assert.Equal(t, lastModified, state["PriorityTimestamp"])
	assert.Equal(t, lastModified, bm["LastModified"])
}

func TestKoboGetState_NoState_EpochPriorityTimestamp(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, bookID := setupKoboSyncBook(t, "kobo-get-epoch-"+uuid.NewString())

	state := koboGetState(t, ts, rawToken, bookID)
	assert.Equal(t, "1970-01-01T00:00:00Z", state["PriorityTimestamp"])
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, bm["Location"])
}

// TestKoboGetState_WebWriteClearsKoboLocation: a percent-only web position
// must not ship a stale span the firmware would jump to.
func TestKoboGetState_WebWriteClearsKoboLocation(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-web-clears-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "2026-09-30T08:15:42Z"))
	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(), owner, bookID, models.ReadingSourceWeb, 55, nil,
	))

	state := koboGetState(t, ts, rawToken, bookID)
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 55.0, bm["ProgressPercent"], 0.01)
	assert.Nil(t, bm["Location"])
}

func TestReadingStateRepo_KoboLocationAndReadAtRoundTrip(t *testing.T) {
	book := addUniqueBook(t)
	ctx := context.Background()
	readAt := time.Date(2026, 9, 30, 8, 15, 42, 0, time.UTC)
	loc := models.KoboLocation{Source: "a.xhtml", Type: "KoboSpan", Value: "kobo.3.4"}

	require.NoError(t, testApp.Repositories.ReadingState.Upsert(ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:       userID,
			BookID:       book.ID,
			Source:       models.ReadingSourceKobo,
			Percent:      12,
			KoboLocation: &loc,
			ReadAt:       &readAt,
		}))

	states, err := testApp.Repositories.ReadingState.ListByUser(ctx, userID)
	require.NoError(t, err)
	var got *models.BookReadingState
	for i := range states {
		if states[i].BookID == book.ID {
			got = &states[i]
		}
	}
	require.NotNil(t, got)
	require.NotNil(t, got.KoboLocation)
	assert.Equal(t, loc, *got.KoboLocation)
	require.NotNil(t, got.ReadAt)
	assert.True(t, readAt.Equal(*got.ReadAt))
}

// TestMergeBooks_CopiedReadingStateDropsKoboLocation: the loser's span need not
// exist in the winner's file, so only the percent carries over.
func TestMergeBooks_CopiedReadingStateDropsKoboLocation(t *testing.T) {
	cleanupMergeUser(t)
	ctx := context.Background()

	winner := addMergeBook(
		t,
		"KoboLocA",
		"9780021570001",
		models.StatusReading,
		[]string{},
	)
	loser := addMergeBook(t, "KoboLocB", "9780021570002", models.StatusReading, []string{})
	require.NoError(t, testApp.Repositories.ReadingState.Upsert(ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:  mergeTestUser,
			BookID:  loser.BookID,
			Source:  models.ReadingSourceKobo,
			Percent: 33,
			KoboLocation: &models.KoboLocation{
				Source: "b.xhtml", Type: "KoboSpan", Value: "kobo.9.9",
			},
		}))

	_, _, err := testApp.Services.Books.MergeBooks(
		ctx, mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, nil, nil,
	)
	require.NoError(t, err)

	got, err := testApp.Repositories.ReadingState.Get(
		ctx, mergeTestUser, winner.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, 33, got.Percent)
	assert.Nil(t, got.KoboLocation)
}
