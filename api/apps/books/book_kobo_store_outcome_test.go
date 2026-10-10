package books_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// putStoreState PUTs a store book's reading state at percent.
func putStoreState(
	t *testing.T,
	ts *httptest.Server,
	rawToken, storeID string,
	percent float64,
) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"ReadingStates": []map[string]any{{
		"EntitlementId": storeID,
		"CurrentBookmark": map[string]any{
			"ProgressPercent": percent,
			"LastModified":    time.Now().UTC().Format(time.RFC3339),
		},
		"StatusInfo": map[string]any{"Status": "Reading"},
	}}})
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+storeID+"/state"), body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// eventuallyStoreOutcome waits for ListKoboStoreBooks to report outcome for
// storeID and returns that entry.
func eventuallyStoreOutcome(
	t *testing.T,
	storeID string,
	outcome models.KoboStoreOutcome,
) *booksv1.KoboStoreBook {
	t.Helper()
	client := newBooksTestClient(t)
	var found *booksv1.KoboStoreBook
	require.Eventually(t, func() bool {
		req := connect.NewRequest(&booksv1.ListKoboStoreBooksRequest{})
		req.Header().Set("Cookie", accessToken.String())
		resp, err := client.ListKoboStoreBooks(context.Background(), req)
		require.NoError(t, err)
		for _, b := range resp.Msg.Books {
			if b.EntitlementId == storeID && b.LastOutcome == string(outcome) {
				found = b
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)
	return found
}

// TestKoboPutState_StoreBook_RecordsMirrorOutcome: each store-book state PUT
// records what mirroring it did, visible through ListKoboStoreBooks.
func TestKoboPutState_StoreBook_RecordsMirrorOutcome(t *testing.T) {
	isbn := testISBN("outcome-" + uuid.NewString())
	title := "Outcome Book " + isbn
	bookID := addStoreLibraryBook(t, userID, title, isbn, "Some Author")

	upstream := koboStoreUpstream(t, nil, []any{
		map[string]any{"Title": title, "Isbn": isbn},
	})
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)
	rawToken := registerTestDevice(t, userID)
	storeID := uuid.NewString()

	putStoreState(t, ts, rawToken, storeID, 30)
	got := eventuallyStoreOutcome(t, storeID, models.KoboStoreMirrored)
	assert.Equal(t, bookID.String(), got.LibraryBookId)
	assert.Equal(t, title, got.LibraryTitle)
	assert.Equal(t, isbn, got.Isbn13)
	require.NotNil(t, got.LastPercent)
	assert.EqualValues(t, 30, *got.LastPercent)
	assert.NotEmpty(t, got.LastReadAt)
	assert.NotEmpty(t, got.LastMirroredAt)

	putStoreState(t, ts, rawToken, storeID, 10)
	eventuallyStoreOutcome(t, storeID, models.KoboStoreNotNewer)

	putStoreState(t, ts, rawToken, storeID, 0)
	got = eventuallyStoreOutcome(t, storeID, models.KoboStoreNoProgress)
	assert.EqualValues(t, 0, *got.LastPercent)

	eventuallyPercent(t, userID, bookID, 30)
}

// TestKoboPutState_StoreBook_FractionalPercentMirrors: progress under half a
// percent still mirrors, as 1%.
func TestKoboPutState_StoreBook_FractionalPercentMirrors(t *testing.T) {
	isbn := testISBN("fraction-" + uuid.NewString())
	title := "Fraction Book " + isbn
	bookID := addStoreLibraryBook(t, userID, title, isbn, "Some Author")

	upstream := koboStoreUpstream(t, nil, []any{
		map[string]any{"Title": title, "Isbn": isbn},
	})
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)
	storeID := uuid.NewString()

	putStoreState(t, ts, registerTestDevice(t, userID), storeID, 0.3)

	got := eventuallyStoreOutcome(t, storeID, models.KoboStoreMirrored)
	require.NotNil(t, got.LastPercent)
	assert.EqualValues(t, 1, *got.LastPercent)
	eventuallyPercent(t, userID, bookID, 1)
}

// TestKoboPutState_UnmatchedStoreBook_RecordsNoMatch: a store book with no
// library match keeps its reported progress and says why it wasn't mirrored.
func TestKoboPutState_UnmatchedStoreBook_RecordsNoMatch(t *testing.T) {
	upstream := koboStoreUpstream(t, nil, []any{map[string]any{
		"Title":        "Not In The Library " + uuid.NewString(),
		"Contributors": []string{"Nobody"},
	}})
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)
	storeID := uuid.NewString()

	putStoreState(t, ts, registerTestDevice(t, userID), storeID, 20)

	got := eventuallyStoreOutcome(t, storeID, models.KoboStoreNoMatch)
	assert.Empty(t, got.LibraryBookId)
	assert.Equal(t, []string{"Nobody"}, got.Authors)
	require.NotNil(t, got.LastPercent)
	assert.EqualValues(t, 20, *got.LastPercent)
}

// TestKoboSync_StoreBook_ListedBeforeAnyMirror: a synced store book is listed
// with its match before a reading state is mirrored.
func TestKoboSync_StoreBook_ListedBeforeAnyMirror(t *testing.T) {
	isbn := testISBN("listed-" + uuid.NewString())
	bookID := addStoreLibraryBook(t, userID, "Listed "+isbn, isbn, "Some Author")
	storeID := uuid.NewString()

	upstream := koboStoreUpstream(t, []any{
		map[string]any{"NewEntitlement": map[string]any{
			"BookEntitlement": map[string]any{
				"Id": storeID, "Accessibility": "Full", "IsRemoved": false,
			},
			"BookMetadata": map[string]any{"Title": "Listed", "Isbn": isbn},
		}},
	}, nil)
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	koboStoreSync(t, ts, registerTestDevice(t, userID))

	got := eventuallyStoreOutcome(t, storeID, models.KoboStoreUnrecorded)
	assert.Equal(t, bookID.String(), got.LibraryBookId)
	assert.True(t, got.Owned)
	assert.Nil(t, got.LastPercent)
	assert.Empty(t, got.LastMirroredAt)
}

// TestConnectListKoboStoreBooks_DBFailure_ReturnsInternal: a failed store-book
// or library read is an error, never an empty list.
func TestConnectListKoboStoreBooks_DBFailure_ReturnsInternal(t *testing.T) {
	isbn := testISBN("listfail-" + uuid.NewString())
	upstream := koboStoreUpstream(t, nil, []any{
		map[string]any{"Title": "Unmatched " + isbn, "Isbn": isbn},
	})
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)
	storeID := uuid.NewString()
	putStoreState(t, ts, registerTestDevice(t, userID), storeID, 5)
	eventuallyStoreOutcome(t, storeID, models.KoboStoreNoMatch)

	for name, failSQL := range map[string]string{
		"store books": "ORDER BY GREATEST(last_mirrored_at, updated_at)",
		"library":     "WHERE ub.user_id = $1\n\t\tORDER BY b.title",
	} {
		t.Run(name, func(t *testing.T) {
			fts := httptest.NewServer(getRoutesWithFailingDB(t, upstream.URL, failSQL))
			t.Cleanup(fts.Close)
			req := connect.NewRequest(&booksv1.ListKoboStoreBooksRequest{})
			req.Header().Set("Cookie", accessToken.String())
			_, err := newBooksClientFor(fts.URL).ListKoboStoreBooks(
				context.Background(), req,
			)
			require.Error(t, err)
			assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		})
	}
}

// TestKoboPutState_OutcomeWriteFails_StillMirrors: failing to record the
// outcome doesn't undo the mirrored progress.
func TestKoboPutState_OutcomeWriteFails_StillMirrors(t *testing.T) {
	owner := "kobo-store-outcomefail-" + uuid.NewString()
	isbn := testISBN(owner)
	bookID := addStoreLibraryBook(t, owner, "Outcome Fail "+owner, isbn, "A")
	upstream := koboStoreUpstream(t, nil, []any{
		map[string]any{"Title": "x", "Isbn": isbn},
	})
	ts := httptest.NewServer(getRoutesWithFailingDB(t, upstream.URL,
		"UPDATE books.kobo_store_books"))
	t.Cleanup(ts.Close)

	putStoreState(t, ts, registerTestDevice(t, owner), uuid.NewString(), 25)

	eventuallyPercent(t, owner, bookID, 25)
}
