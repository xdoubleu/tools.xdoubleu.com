package books_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

var errInjectedDB = errors.New("injected db failure")

// failingDB is testDB with every statement containing failSQL failing.
type failingDB struct {
	postgres.DB
	failSQL string
}

func (f failingDB) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	if strings.Contains(sql, f.failSQL) {
		return pgconn.CommandTag{}, errInjectedDB
	}
	return f.DB.Exec(ctx, sql, args...)
}

func (f failingDB) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (pgx.Rows, error) {
	if strings.Contains(sql, f.failSQL) {
		return nil, errInjectedDB
	}
	return f.DB.Query(ctx, sql, args...)
}

func (f failingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, f.failSQL) {
		return errRow{}
	}
	return f.DB.QueryRow(ctx, sql, args...)
}

type errRow struct{}

func (errRow) Scan(...any) error { return errInjectedDB }

// getRoutesWithFailingDB is getRoutesWithKoboUpstream over a failingDB.
func getRoutesWithFailingDB(t *testing.T, upstreamURL, failSQL string) http.Handler {
	t.Helper()
	clients := books.Clients{
		UniCat:           nil,
		Hardcover:        mocks.NewMockHardcoverClient(),
		ObjectStore:      objectstore.NewFake(),
		WebFetch:         nil,
		KoboStoreBaseURL: upstreamURL,
		PublicAPIBaseURL: "",
	}
	app := books.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		failingDB{DB: testDB, failSQL: failSQL},
		clients,
	)
	return testhelper.BuildMux(app)
}

// addStoreLibraryBook adds a library book for owner; isbn may be empty.
func addStoreLibraryBook(
	t *testing.T,
	owner, title, isbn, author string,
) uuid.UUID {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct // minimal book
		Source:  "manual",
		Title:   title,
		Authors: []string{author},
		ISBN13:  isbn,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), owner, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	return ub.BookID
}

// koboStoreUpstream serves syncItems on /v1/library/sync, meta on
// /v1/library/{id}/metadata (404 when nil) and accepts state PUTs.
func koboStoreUpstream(t *testing.T, syncItems []any, meta []any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/v1/library/sync":
				_ = json.NewEncoder(w).Encode(syncItems)
			case r.Method == http.MethodPut:
				_, _ = w.Write([]byte(`{"RequestResult":"Success"}`))
			case meta != nil && r.Method == http.MethodGet:
				_ = json.NewEncoder(w).Encode(meta)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	)
	t.Cleanup(srv.Close)
	return srv
}

func storeEntitlement(
	id string,
	ent map[string]any,
	meta map[string]any,
	percent int,
) map[string]any {
	e := map[string]any{"Id": id, "Accessibility": "Full", "IsRemoved": false}
	for k, v := range ent {
		e[k] = v
	}
	return map[string]any{"NewEntitlement": map[string]any{
		"BookEntitlement": e,
		"BookMetadata":    meta,
		"ReadingState": map[string]any{
			"EntitlementId": id,
			"CurrentBookmark": map[string]any{
				"ProgressPercent": percent,
				"LastModified":    time.Now().UTC().Format(time.RFC3339),
			},
			"StatusInfo": map[string]any{"Status": "Reading"},
		},
	}}
}

func koboStoreSync(t *testing.T, ts *httptest.Server, rawToken string) {
	t.Helper()
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/sync"), nil))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func hasOwnBol(t *testing.T, owner string, bookID uuid.UUID) bool {
	t.Helper()
	ub, err := testApp.Services.Books.GetUserBook(context.Background(), owner, bookID)
	require.NoError(t, err)
	return ub.HasTag(models.TagOwnBol)
}

func eventuallyPercent(t *testing.T, owner string, bookID uuid.UUID, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		st, err := testApp.Services.Books.GetReadingState(
			context.Background(), owner, bookID,
		)
		return err == nil && st.Percent == want
	}, 5*time.Second, 20*time.Millisecond)
}

// TestKoboSync_OwnedStoreBook_TagsOwnBolAndMirrorsProgress: a purchased store
// book matched by ISBN gets own-bol and its store progress.
func TestKoboSync_OwnedStoreBook_TagsOwnBolAndMirrorsProgress(t *testing.T) {
	owner := "kobo-store-owned-" + uuid.NewString()
	isbn := testISBN(owner)
	bookID := addStoreLibraryBook(t, owner, "Store Book "+owner, isbn, "Some Author")

	storeID := uuid.NewString()
	upstream := koboStoreUpstream(t, []any{
		storeEntitlement(storeID, nil,
			map[string]any{"Title": "Other Title", "Isbn": "978-" + isbn[3:]}, 42),
	}, nil)
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	koboStoreSync(t, ts, registerTestDevice(t, owner))

	eventuallyPercent(t, owner, bookID, 42)
	assert.True(t, hasOwnBol(t, owner, bookID))
}

// TestKoboSync_SubscriptionStoreBook_NotOwned: a time-limited entitlement
// (Kobo Plus, loans) mirrors progress but isn't tagged owned.
func TestKoboSync_SubscriptionStoreBook_NotOwned(t *testing.T) {
	owner := "kobo-store-sub-" + uuid.NewString()
	isbn := testISBN(owner)
	bookID := addStoreLibraryBook(t, owner, "Sub Book "+owner, isbn, "Some Author")

	upstream := koboStoreUpstream(t, []any{
		storeEntitlement(uuid.NewString(),
			map[string]any{"ActivePeriod": map[string]string{
				"From": "2026-01-01T00:00:00Z", "To": "2026-12-01T00:00:00Z",
			}},
			map[string]any{"Title": "x", "Isbn": isbn}, 15),
	}, nil)
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	koboStoreSync(t, ts, registerTestDevice(t, owner))

	eventuallyPercent(t, owner, bookID, 15)
	assert.False(t, hasOwnBol(t, owner, bookID))
}

// TestKoboSync_StoreBookByTitleAndAuthor_ChangedReadingState: a store book
// without an ISBN match links by title and author, and a ChangedReadingState
// is mirrored.
func TestKoboSync_StoreBookByTitleAndAuthor_ChangedReadingState(t *testing.T) {
	owner := "kobo-store-title-" + uuid.NewString()
	title := "The Title Match " + owner
	bookID := addStoreLibraryBook(t, owner, title, "", "Jane Doe")

	storeID := uuid.NewString()
	upstream := koboStoreUpstream(t, []any{
		storeEntitlement(storeID, nil, map[string]any{
			"Title":            title,
			"Isbn":             "n/a",
			"ContributorRoles": []map[string]string{{"Name": "Jane Doe", "Role": "Author"}},
		}, 0),
		map[string]any{"ChangedReadingState": map[string]any{
			"ReadingState": map[string]any{
				"EntitlementId": storeID,
				"CurrentBookmark": map[string]any{
					"ProgressPercent": 55,
					"LastModified":    time.Now().UTC().Format(time.RFC3339),
				},
			},
		}},
	}, nil)
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	koboStoreSync(t, ts, registerTestDevice(t, owner))

	eventuallyPercent(t, owner, bookID, 55)
	assert.True(t, hasOwnBol(t, owner, bookID))
}

// TestKoboPutState_UnseenStoreBook_LearnsMetadataAndMirrors: a state PUT for
// a store book not yet seen in a sync looks up its metadata, then mirrors.
// Ownership is unknown from metadata, so no own-bol.
func TestKoboPutState_UnseenStoreBook_LearnsMetadataAndMirrors(t *testing.T) {
	owner := "kobo-store-put-" + uuid.NewString()
	isbn := testISBN(owner)
	bookID := addStoreLibraryBook(t, owner, "Put Book "+owner, isbn, "Some Author")

	upstream := koboStoreUpstream(t, nil, []any{
		map[string]any{"Title": "Put Book", "Isbn": isbn},
	})
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)
	rawToken := registerTestDevice(t, owner)

	body, err := json.Marshal(map[string]any{"ReadingStates": []map[string]any{{
		"CurrentBookmark": map[string]any{
			"ProgressPercent": 30,
			"LastModified":    time.Now().UTC().Format(time.RFC3339),
		},
		"StatusInfo": map[string]any{"Status": "Finished"},
	}}})
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+uuid.NewString()+"/state"), body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	eventuallyPercent(t, owner, bookID, models.MaxProgressPercent)
	assert.False(t, hasOwnBol(t, owner, bookID))
}

// TestKoboPutState_StoreBookNoMetadata_LeavesLibraryAlone: an upstream
// metadata failure only skips the mirror.
func TestKoboPutState_StoreBookNoMetadata_LeavesLibraryAlone(t *testing.T) {
	owner := "kobo-store-nometa-" + uuid.NewString()
	bookID := addStoreLibraryBook(t, owner, "No Meta "+owner, testISBN(owner), "A")

	upstream := koboStoreUpstream(t, nil, nil)
	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	body := []byte(`{"ReadingStates":[{"CurrentBookmark":{"ProgressPercent":20}}]}`)
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, registerTestDevice(t, owner),
			"/v1/library/"+uuid.NewString()+"/state"), body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	time.Sleep(200 * time.Millisecond)
	_, err = testApp.Services.Books.GetReadingState(context.Background(), owner, bookID)
	assert.Error(t, err)
}

// TestKoboState_LibraryLookupFails_Returns500: a failed library lookup is an
// error, never mistaken for a store book.
func TestKoboState_LibraryLookupFails_Returns500(t *testing.T) {
	upstream := koboStoreUpstream(t, nil, nil)
	ts := httptest.NewServer(getRoutesWithFailingDB(t, upstream.URL,
		"WHERE ub.user_id = $1 AND ub.book_id = $2"))
	t.Cleanup(ts.Close)
	rawToken := registerTestDevice(t, "kobo-store-500-"+uuid.NewString())
	id := uuid.NewString()

	for _, req := range []*http.Request{
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/"+id+"/state"), nil),
		koboReq(t, http.MethodPut,
			koboURL(ts, rawToken, "/v1/library/"+id+"/state"), []byte(`{}`)),
	} {
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, req.URL.Path)
	}
}

// TestKoboSync_StoreDBFailures_LeaveLibraryAlone: a failing step while linking
// or mirroring store books changes nothing it shouldn't.
func TestKoboSync_StoreDBFailures_LeaveLibraryAlone(t *testing.T) {
	notOwned := map[string]any{"ActivePeriod": map[string]string{"To": "x"}}
	//nolint:exhaustruct // cases set only the fields they vary
	for name, tc := range map[string]struct {
		failSQL  string
		ent      map[string]any
		noISBN   bool
		wantsBol bool
	}{
		"upsert":      {failSQL: "INSERT INTO books.kobo_store_books"},
		"isbn lookup": {failSQL: "AND b.isbn13 = $2"},
		"library": {
			failSQL: "WHERE ub.user_id = $1\n\t\tORDER BY b.title", noISBN: true,
		},
		"tag":           {failSQL: "SET tags = $3"},
		"store lookup":  {failSQL: "FROM books.kobo_store_books", wantsBol: true},
		"reading match": {failSQL: "AND b.isbn13 = $2", ent: notOwned},
	} {
		t.Run(name, func(t *testing.T) {
			owner := "kobo-store-fail-" + uuid.NewString()
			isbn := testISBN(owner)
			bookID := addStoreLibraryBook(t, owner, "Fail "+owner, isbn, "Author")
			meta := map[string]any{"Title": "Fail " + owner, "Isbn": isbn}
			if tc.noISBN {
				meta = map[string]any{"Title": "Fail " + owner, "Contributors": []string{"Author"}}
			}

			upstream := koboStoreUpstream(t,
				[]any{storeEntitlement(uuid.NewString(), tc.ent, meta, 40)}, nil)
			ts := httptest.NewServer(getRoutesWithFailingDB(t, upstream.URL, tc.failSQL))
			t.Cleanup(ts.Close)

			koboStoreSync(t, ts, registerTestDevice(t, owner))

			if tc.wantsBol {
				require.Eventually(t, func() bool { return hasOwnBol(t, owner, bookID) },
					5*time.Second, 20*time.Millisecond)
			} else {
				time.Sleep(200 * time.Millisecond)
				assert.False(t, hasOwnBol(t, owner, bookID))
			}
			_, err := testApp.Services.Books.GetReadingState(
				context.Background(), owner, bookID,
			)
			assert.Error(t, err)
		})
	}
}
