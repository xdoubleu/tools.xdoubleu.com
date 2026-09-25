package books_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
)

// registerTestDevice registers a Kobo device for ownerID and returns its raw token.
func registerTestDevice(t *testing.T, ownerID string) string {
	t.Helper()
	_, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		context.Background(), ownerID, "Test Kobo", "",
	)
	require.NoError(t, err)
	return rawToken
}

// TestKoboProxy_UnhandledPathProxied: unowned paths go verbatim upstream.
func TestKoboProxy_UnhandledPathProxied(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`"firmware-ok"`))
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-proxy-unhandled-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/UpgradeCheck"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestKoboProxy_TokenStrippedFromUpstreamPath: upstream gets /v1/…, not
// /{token}/v1/…, which the real store rejects with 401.
func TestKoboProxy_TokenStrippedFromUpstreamPath(t *testing.T) {
	var capturedPath string
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-proxy-strip-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/auth/device"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/v1/auth/device", capturedPath,
		"upstream must receive /v1/auth/device without the token segment")
}

// TestKoboProxy_InvalidToken_Returns401 checks token auth runs before proxying.
func TestKoboProxy_InvalidToken_Returns401(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet,
			koboURL(ts, "not-a-registered-token", "/v1/UpgradeCheck"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestKoboLibrarySync_MergesUpstreamItems checks upstream items are kept.
func TestKoboLibrarySync_MergesUpstreamItems(t *testing.T) {
	const upstreamRevID = "upstream-book-001"
	upstreamPayload := `[{"BookEntitlement":` +
		`{"RevisionId":"` + upstreamRevID + `","Id":"` + upstreamRevID + `",` +
		`"Status":"Active","Type":"ebook","Accessibility":"Full",` +
		`"ActivePeriod":{},"Created":"2024-01-01T00:00:00Z",` +
		`"CrossRevisionId":"` + upstreamRevID + `","IsRemoved":false,` +
		`"IsHiddenFromUI":false,"PurchasedDate":"2024-01-01T00:00:00Z"},` +
		`"BookMetadata":{"Title":"Upstream Book",` +
		`"ContentType":"application/x-kobo-epub+zip",` +
		`"RevisionId":"` + upstreamRevID + `","Language":"en"},` +
		`"DownloadUrls":[],"ReadingState":null}]`

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(upstreamPayload))
		}),
	)
	t.Cleanup(upstream.Close)

	owner := "kobo-merge-user-" + uuid.NewString()
	rawToken := registerTestDevice(t, owner)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	found := false
	for _, e := range entries {
		if ent, ok := e["BookEntitlement"].(map[string]any); ok {
			if ent["RevisionId"] == upstreamRevID {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "upstream item must appear in merged sync response")
}

// TestKoboProxy_UpstreamDown_ReturnsBadGateway checks the 502 when upstream is down.
func TestKoboProxy_UpstreamDown_ReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(
		getRoutesWithKoboUpstream(t, "http://127.0.0.1:0"),
	)
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-proxy-down-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/UpgradeCheck"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

// TestKoboLibrarySync_UpstreamNon200_FallsBackToOurBooks checks graceful degradation.
func TestKoboLibrarySync_UpstreamNon200_FallsBackToOurBooks(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	t.Cleanup(upstream.Close)

	owner := "kobo-non200-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	found := false
	for _, e := range entries {
		ne, neOK := e["NewEntitlement"].(map[string]any)
		if !neOK {
			continue
		}
		if ent, entOK := ne["BookEntitlement"].(map[string]any); entOK {
			if ent["Id"] == bookID.String() {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "our book must appear even when upstream returns non-200")
}

// TestKoboLibrarySync_ForwardsSyncToken checks x-kobo-sync headers are forwarded.
func TestKoboLibrarySync_ForwardsSyncToken(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("x-kobo-sync", "continuation-abc")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}),
	)
	t.Cleanup(upstream.Close)

	rawToken := registerTestDevice(t, "kobo-synctoken-"+uuid.NewString())

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "continuation-abc", resp.Header.Get("x-kobo-sync"))
}

// TestKoboLibrarySync_OurBooksPreservedWhenUpstreamDown: our books survive an outage.
func TestKoboLibrarySync_OurBooksPreservedWhenUpstreamDown(t *testing.T) {
	owner := "kobo-upstream-down-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	ts := httptest.NewServer(
		getRoutesWithKoboUpstream(t, "http://127.0.0.1:0"),
	)
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	found := false
	for _, e := range entries {
		ne, neOK := e["NewEntitlement"].(map[string]any)
		if !neOK {
			continue
		}
		if ent, entOK := ne["BookEntitlement"].(map[string]any); entOK {
			if ent["Id"] == bookID.String() {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "our book must still appear when upstream is down")
}

// TestKoboLibrarySync_EmptyLibraryAndUpstreamDown: the body must be [] not
// null (the firmware hangs on null). Checked as raw bytes because decoding
// turns null into an empty slice.
func TestKoboLibrarySync_EmptyLibraryAndUpstreamDown(t *testing.T) {
	rawToken := registerTestDevice(t, "kobo-empty-and-down-"+uuid.NewString())

	ts := httptest.NewServer(
		getRoutesWithKoboUpstream(t, "http://127.0.0.1:0"),
	)
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "[]\n", string(body))
}

// koboURL builds a device-shaped URL: <server>/books/kobo/<rawToken><path>.
func koboURL(ts *httptest.Server, rawToken, path string) string {
	return ts.URL + "/books/kobo/" + rawToken + path
}

// koboReq builds a request with X-Forwarded-Proto: https.
func koboReq(t *testing.T, method, url string, body []byte) *http.Request {
	t.Helper()
	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequestWithContext(
			context.Background(), method, url, bytes.NewReader(body),
		)
	} else {
		req, err = http.NewRequestWithContext(context.Background(), method, url, nil)
	}
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")
	return req
}

// setupKoboSyncBook creates a kobo-sync book with a ready KEPUB and returns
// (rawToken, bookID). ownerID must be unique per run.
func setupKoboSyncBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	_, bookID := uploadFileForOwner(t, ownerID, models.FileFormatEPUB)
	_, err := testApp.Services.Conversion.EnsureKEPUB(
		context.Background(), ownerID, bookID,
	)
	require.NoError(t, err)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), ownerID, bookID,
	))
	rawToken := registerTestDevice(t, ownerID)
	return rawToken, bookID
}

// setupKoboPDFSyncBook creates a kobo-sync PDF book served as raw PDF (no
// KEPUB row) and returns (rawToken, bookID).
func setupKoboPDFSyncBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	_, bookID := uploadFileForOwner(t, ownerID, models.FileFormatPDF)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), ownerID, bookID,
	))
	err := testApp.Repositories.Books.UpdateTags(
		context.Background(), ownerID, bookID,
		[]string{models.TagKoboSync, models.TagKoboFormatPDF},
		true, // has kobo-sync tag
	)
	require.NoError(t, err)
	rawToken := registerTestDevice(t, ownerID)
	return rawToken, bookID
}

func TestKoboInit_UnregisteredToken_Returns401(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost,
			koboURL(ts, "not-a-registered-token", "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestKoboInit_InvalidToken_Returns401(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(
			t,
			http.MethodPost,
			koboURL(ts, "bad-token-xyz", "/v1/initialization"),
			nil,
		),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestKoboInit_NonHTTPS_Rejected(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-https-test-user")

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		koboURL(ts, rawToken, "/v1/initialization"), nil,
	)
	require.NoError(t, err)
	// Deliberately NOT setting X-Forwarded-Proto: https

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestKoboInit_ValidToken_ReturnsInitData(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-init-ok-user")

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost, koboURL(ts, rawToken, "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "TokenList")
	assert.Contains(t, body, "Settings")
}

func TestKoboLibrarySync_EmptyLibrary(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-sync-empty-user")

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	assert.Empty(t, entries)
}

func TestKoboLibrarySync_ConvertingKEPUBSkipped(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-sync-converting-user"
	rawToken := registerTestDevice(t, owner)

	// No ready KEPUB row: simulates a book still converting.
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), owner, bookID,
	))

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	assert.Empty(t, entries)
}

func TestKoboLibrarySync_ReadyKEPUBIncluded(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-ready-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok, "entry must be wrapped under NewEntitlement")

	entitlement, ok := ne["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bookID.String(), entitlement["Id"])
	assert.Equal(t, bookID.String(), entitlement["RevisionId"])
	assert.Equal(t, bookID.String(), entitlement["CrossRevisionId"])

	meta, ok := ne["BookMetadata"].(map[string]any)
	require.True(t, ok)
	dlUrls, ok := meta["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KEPUB", dl["Format"])
	// "Desktop" isn't in the device's DownloadUrlFilter=Generic,Android and is
	// silently dropped.
	assert.Equal(t, "Generic", dl["Platform"])
	dlURL, ok := dl["Url"].(string)
	require.True(t, ok)
	assert.Contains(t, dlURL, bookID.String()+"/file")
}

// TestKoboLibrarySync_StaleKEPUB_TriggersRegeneration: sync regenerates a KEPUB
// from an older converter version.
func TestKoboLibrarySync_StaleKEPUB_TriggersRegeneration(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-stale-regen-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	_, err := testDB.Exec(context.Background(),
		`UPDATE books.book_files SET converter_version = 0
		 WHERE book_id = $1 AND user_id = $2 AND format = 'kepub'`,
		bookID, owner,
	)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool {
		var version int16
		scanErr := testDB.QueryRow(context.Background(),
			`SELECT converter_version FROM books.book_files
			 WHERE book_id = $1 AND user_id = $2 AND format = 'kepub'`,
			bookID, owner,
		).Scan(&version)
		return scanErr == nil && version == services.CurrentKEPUBConverterVersion()
	}, 5*time.Second, 50*time.Millisecond,
		"a stale KEPUB must be regenerated after a library sync")
}

// TestKoboLibrarySync_UnchangedRevision_StaysNewEntitlement: an unchanged
// resync keeps NewEntitlement.
func TestKoboLibrarySync_UnchangedRevision_StaysNewEntitlement(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-unchanged-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	firstResp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	firstResp.Body.Close()

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok, "unchanged resync must still use NewEntitlement")
	entitlement, ok := ne["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bookID.String(), entitlement["Id"])
}

// TestKoboLibrarySync_RevisionChanged_UsesChangedEntitlement: a converter bump
// on a synced book is sent as ChangedEntitlement so the device invalidates
// rather than duplicates it.
func TestKoboLibrarySync_RevisionChanged_UsesChangedEntitlement(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-changed-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	firstResp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	firstResp.Body.Close()

	// Bump the converter version directly; only the discriminator matters here.
	_, err = testDB.Exec(context.Background(),
		`UPDATE books.book_files SET converter_version = converter_version + 1
		 WHERE book_id = $1 AND user_id = $2 AND format = 'kepub'`,
		bookID, owner,
	)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	_, isNew := entries[0]["NewEntitlement"]
	assert.False(t, isNew, "a changed revision must not be sent as NewEntitlement")

	ce, ok := entries[0]["ChangedEntitlement"].(map[string]any)
	require.True(t, ok, "a changed revision must be wrapped in ChangedEntitlement")
	entitlement, ok := ce["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(
		t,
		bookID.String(),
		entitlement["Id"],
		"Id must stay the bare book UUID so progress/EntitlementId resolution is unaffected",
	)

	// Only the transition uses ChangedEntitlement.
	thirdResp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer thirdResp.Body.Close()

	var thirdEntries []map[string]any
	require.NoError(t, json.NewDecoder(thirdResp.Body).Decode(&thirdEntries))
	require.Len(t, thirdEntries, 1)
	_, isNewAgain := thirdEntries[0]["NewEntitlement"]
	assert.True(t, isNewAgain,
		"a subsequent unchanged sync must return to NewEntitlement")
}

// TestKoboBackfilledConverterVersion_ThenRegeneration_UsesChangedEntitlement:
// a row stamped the way migrations 00016/00018/00019 leave it must count as
// already synced, so regeneration yields ChangedEntitlement. The stamp is
// applied by hand since goose ran before this data existed.
func TestKoboBackfilledConverterVersion_ThenRegeneration_UsesChangedEntitlement(
	t *testing.T,
) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-backfill-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	_, err := testDB.Exec(context.Background(),
		`UPDATE books.user_books ub
		 SET kobo_last_synced_converter_version = bf.converter_version
		 FROM books.book_files bf
		 WHERE bf.book_id = ub.book_id
		   AND bf.user_id = ub.user_id
		   AND bf.status = 'ready'
		   AND bf.format = CASE
		       WHEN 'kobo-format-pdf' = ANY(ub.tags) THEN 'pdf'
		       ELSE 'kepub'
		   END
		   AND ub.book_id = $1 AND ub.user_id = $2`,
		bookID, owner,
	)
	require.NoError(t, err)

	var backfilled *int16
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT kobo_last_synced_converter_version FROM books.user_books
		 WHERE book_id = $1 AND user_id = $2`,
		bookID, owner,
	).Scan(&backfilled))
	require.NotNil(t, backfilled,
		"backfill must stamp an already-enabled book's converter version")

	_, err = testDB.Exec(context.Background(),
		`UPDATE books.book_files SET converter_version = converter_version + 1
		 WHERE book_id = $1 AND user_id = $2 AND format = 'kepub'`,
		bookID, owner,
	)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	_, isNew := entries[0]["NewEntitlement"]
	assert.False(t, isNew,
		"a backfilled, already-on-device book must not resync as NewEntitlement")

	ce, ok := entries[0]["ChangedEntitlement"].(map[string]any)
	require.True(t, ok, "a regenerated book must be wrapped in ChangedEntitlement")
	entitlement, ok := ce["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(
		t,
		bookID.String(),
		entitlement["Id"],
		"Id must stay the bare book UUID so the firmware updates in place",
	)
}

func TestKoboLibrarySync_UserACannotSeeUserBBooks(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	userB := "kobo-iso-b-" + uuid.NewString()
	_, _ = setupKoboSyncBook(t, userB)

	rawTokenA := registerTestDevice(t, "kobo-iso-a-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawTokenA, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	assert.Empty(t, entries, "user A must not see user B's books")
}

func TestKoboFile_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-file-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/not-a-uuid/file"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboGetState_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-gstate-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/not-a-uuid/state"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboPutState_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-pstate-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/not-a-uuid/state"), []byte(`{}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboPutState_BadJSON(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-putstate-badjson-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"),
		[]byte(`not-json`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboFile_Download_Redirect(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-file-dl-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req := koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/file"), nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Location"))
}

func TestKoboFile_UserBCannotDownloadUserAFile(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const userA = "kobo-file-iso-user-a"
	_, bookID := setupKoboSyncBook(t, userA)

	rawTokenB := registerTestDevice(t, "kobo-file-iso-user-b")

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawTokenB, "/v1/library/"+bookID.String()+"/file"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestKoboState_GetNoState(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-get-nostate-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)

	// Epoch, so the device's local progress wins and gets pushed.
	assert.Equal(t, "1970-01-01T00:00:00Z", state["LastModified"],
		"no-state LastModified must be epoch so device wins conflict and pushes")
	si, ok := state["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1970-01-01T00:00:00Z", si["LastModified"])
}

// TestKoboState_PutLocationEmptyValueOmitted: an empty Location Value means
// no location.
func TestKoboState_PutLocationEmptyValueOmitted(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-empty-location-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	body, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{
				"ProgressPercent": 10,
				"Location": map[string]any{
					"Source": "x",
					"Type":   "KoboSpan",
					"Value":  "",
				},
			},
		}},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(putResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, bm["Location"], "empty Value must not be stored as a location")
}

func TestKoboState_PutThenGetRoundTrip(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-roundtrip-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	location := "kobo.22.1"
	// Real device shape.
	body, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{
				"ContentSourceProgressPercent": 0,
				"ProgressPercent":              1,
				"Location": map[string]any{
					"Source": "index_split_000.html",
					"Type":   "KoboSpan",
					"Value":  location,
				},
			},
			"StatusInfo": map[string]any{"Status": "Reading"},
		}},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(t, http.StatusOK, putResp.StatusCode)

	getResp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), nil))
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 1.0, bm["ProgressPercent"], 0.01)
	assert.InDelta(t, 1.0, bm["ContentSourceProgressPercent"], 0.01)
	assert.Equal(t, location, bm["Location"])
	si, ok := state["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Reading", si["Status"])
}

// TestKoboState_PutEmptyReadingStates: an empty array must not reset progress.
func TestKoboState_PutEmptyReadingStates(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-empty-put-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	seedBody, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 50},
		}},
	})
	require.NoError(t, err)
	seedResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), seedBody))
	require.NoError(t, err)
	seedResp.Body.Close()
	require.Equal(t, http.StatusOK, seedResp.StatusCode)

	emptyBody, err := json.Marshal(map[string]any{"ReadingStates": []map[string]any{}})
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), emptyBody))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(putResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 50.0, bm["ProgressPercent"], 0.01,
		"empty ReadingStates must preserve existing progress, not zero it")
}

// TestKoboState_PutLowerProgress_DoesNotRegress: a stale device PUT with a
// lower percent must not clobber a previously synced position.
func TestKoboState_PutLowerProgress_DoesNotRegress(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-regression-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	// Seed a higher progress value first.
	seedBody, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 80},
		}},
	})
	require.NoError(t, err)
	seedResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), seedBody))
	require.NoError(t, err)
	seedResp.Body.Close()
	require.Equal(t, http.StatusOK, seedResp.StatusCode)

	// A stale device PUT reporting a lower percent must not regress it.
	staleBody, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 5},
		}},
	})
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), staleBody))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(putResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 80.0, bm["ProgressPercent"], 0.01,
		"a lower device-reported percent must not regress existing progress")
}

// TestKoboState_PutZeroProgressReportsReadyToRead covers the non-nil 0% state,
// distinct from the hardcoded never-synced case.
func TestKoboState_PutZeroProgressReportsReadyToRead(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-zero-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	body, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 0},
		}},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(putResp.Body).Decode(&state))
	si, ok := state["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ReadyToRead", si["Status"])
}

// TestKoboState_PutFullProgressReportsFinished checks 100% reads as "Finished".
func TestKoboState_PutFullProgressReportsFinished(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-finished-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	body, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 100},
			"StatusInfo":      map[string]any{"Status": "Closed"},
		}},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(putResp.Body).Decode(&state))
	si, ok := state["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Finished", si["Status"])
}

// TestKoboLibrarySync_ReadingStateIncluded: every entry needs a ReadingState
// or the device never PUTs progress.
func TestKoboLibrarySync_ReadingStateIncluded(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-state-present-" + uuid.NewString()
	rawToken, _ := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok, "entry must be wrapped under NewEntitlement")

	rs, ok := ne["ReadingState"].(map[string]any)
	require.True(t, ok, "ReadingState must be non-nil so the firmware syncs progress")

	bm, ok := rs["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, bm["ProgressPercent"], 0.001,
		"new book with no progress should advertise 0")

	assert.Equal(t, "1970-01-01T00:00:00Z", rs["LastModified"],
		"no-state ReadingState.LastModified must be epoch so device pushes progress")
}

// TestKoboLibrarySync_ReadingStateReflectsProgress checks sync echoes pushed progress.
func TestKoboLibrarySync_ReadingStateReflectsProgress(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-state-progress-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)
	location := "epubcfi(/6/4[chap01]!/4/2/1:0)"

	putBody, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{
				"ProgressPercent": 65,
				"Location": map[string]any{
					"Source": "index_split_000.html",
					"Type":   "KoboSpan",
					"Value":  location,
				},
			},
			"StatusInfo": map[string]any{"Status": "Reading"},
		}},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), putBody))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	syncResp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer syncResp.Body.Close()
	assert.Equal(t, http.StatusOK, syncResp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(syncResp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok)

	rs, ok := ne["ReadingState"].(map[string]any)
	require.True(t, ok, "ReadingState must be present")

	bm, ok := rs["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 65.0, bm["ProgressPercent"], 0.01)
	assert.Equal(t, location, bm["Location"])
	si, ok := rs["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Reading", si["Status"],
		"sync manifest must reflect the device's in-progress status")
}

// TestKoboLibrarySync_PDFFormat_ServesPDF checks kobo-format-pdf advertises PDF.
func TestKoboLibrarySync_PDFFormat_ServesPDF(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-pdf-sync-" + uuid.NewString()
	rawToken, bookID := setupKoboPDFSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok, "entry must be wrapped under NewEntitlement")

	meta, ok := ne["BookMetadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "application/pdf", meta["ContentType"])

	dlUrls, ok := meta["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "PDF", dl["Format"])
	dlURL, ok := dl["Url"].(string)
	require.True(t, ok)
	assert.Contains(t, dlURL, bookID.String()+"/file")
}

// TestKoboFile_PDFFormat_RedirectsToPDF checks kobo-format-pdf serves the PDF.
func TestKoboFile_PDFFormat_RedirectsToPDF(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-pdf-file-" + uuid.NewString()
	rawToken, bookID := setupKoboPDFSyncBook(t, owner)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req := koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/file"), nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Location"))
}

func TestKoboState_UserBCannotReadUserAState(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const userA = "kobo-state-iso-user-a"
	_, bookID := setupKoboSyncBook(t, userA)

	rawTokenA := registerTestDevice(t, userA)
	body, err := json.Marshal(map[string]any{
		"ReadingStates": []map[string]any{{
			"CurrentBookmark": map[string]any{"ProgressPercent": 90},
			"StatusInfo":      map[string]any{"Status": "Reading"},
		}},
	})
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawTokenA, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	rawTokenB := registerTestDevice(t, "kobo-state-iso-user-b")

	getResp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawTokenB, "/v1/library/"+bookID.String()+"/state"), nil))
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)
}

func TestKoboMetadata_ReturnsDownloadURL(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-meta-kepub-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var metas []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metas))
	require.Len(t, metas, 1)

	meta := metas[0]
	assert.Equal(t, bookID.String(), meta["RevisionId"])
	assert.Equal(t, "application/x-kobo-epub+zip", meta["ContentType"])

	dlUrls, ok := meta["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KEPUB", dl["Format"])
	assert.Equal(t, "Generic", dl["Platform"])
	dlURL, ok := dl["Url"].(string)
	require.True(t, ok)
	assert.Contains(t, dlURL, bookID.String()+"/file")
}

func TestKoboMetadata_PDF(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-meta-pdf-user"
	rawToken, bookID := setupKoboPDFSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var metas []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metas))
	require.Len(t, metas, 1)

	dl, ok := metas[0]["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dl, 1)
	entry, ok := dl[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "PDF", entry["Format"])
	assert.Equal(t, "application/pdf", metas[0]["ContentType"])
}

func TestKoboMetadata_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-meta-invalid-"+uuid.NewString())
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/not-a-uuid/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestKoboMetadata_UnknownBookProxiedUpstream: a book without a kobo-sync row
// is proxied rather than answered locally.
func TestKoboMetadata_UnknownBookProxiedUpstream(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"RevisionId":"upstream-book"}]`))
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-meta-proxy-"+uuid.NewString())
	unknownID := uuid.New()

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+unknownID.String()+"/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestKoboLibrarySync_EntitlementStableAcrossSyncs: entitlement timestamps must
// be identical across syncs or the firmware recreates it (books flicker).
func TestKoboLibrarySync_EntitlementStableAcrossSyncs(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-stable-ts-" + uuid.NewString()
	rawToken, _ := setupKoboSyncBook(t, owner)

	syncOnce := func() map[string]any {
		resp, err := http.DefaultClient.Do(
			koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var entries []map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
		require.Len(t, entries, 1)

		ne, ok := entries[0]["NewEntitlement"].(map[string]any)
		require.True(t, ok)
		ent, ok := ne["BookEntitlement"].(map[string]any)
		require.True(t, ok)
		return ent
	}

	first := syncOnce()
	second := syncOnce()

	assert.Equal(t, first["Created"], second["Created"],
		"Created must be identical across syncs")
	assert.Equal(t, first["PurchasedDate"], second["PurchasedDate"],
		"PurchasedDate must be identical across syncs")

	ap1, ok1 := first["ActivePeriod"].(map[string]any)
	ap2, ok2 := second["ActivePeriod"].(map[string]any)
	require.True(t, ok1 && ok2)
	assert.Equal(t, ap1["From"], ap2["From"],
		"ActivePeriod.From must be identical across syncs")

	// A varying revision makes the firmware add a duplicate entitlement.
	assert.Equal(t, first["RevisionId"], second["RevisionId"],
		"RevisionId must be identical across syncs with no version change")
	assert.Equal(t, first["CrossRevisionId"], second["CrossRevisionId"],
		"CrossRevisionId must be identical across syncs with no version change")
}

// TestKoboLibrarySync_EntitlementTimestampIsEnableTime: timestamps are the
// kobo-sync enable time, not the request time.
func TestKoboLibrarySync_EntitlementTimestampIsEnableTime(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-enable-ts-" + uuid.NewString()

	// Created is second-precision RFC3339.
	before := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	rawToken, bookID := setupKoboSyncBook(t, owner)
	after := time.Now().UTC().Add(time.Second).Truncate(time.Second)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok)
	ent, ok := ne["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bookID.String(), ent["RevisionId"])

	created, ok := ent["Created"].(string)
	require.True(t, ok, "Created must be a string timestamp")
	ts2, err := time.Parse(time.RFC3339, created)
	require.NoError(t, err, "Created must be a valid RFC3339 timestamp")

	assert.True(t, !ts2.Before(before) && !ts2.After(after),
		"Created must equal the kobo-sync enable time, got %s (window: %s–%s)",
		ts2, before, after)
}

// TestKoboLibrarySync_DisabledBook_EmitsRemoval: disabling a synced book must
// emit ChangedEntitlement with IsRemoved:true.
func TestKoboLibrarySync_DisabledBook_EmitsRemoval(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-disabled-removal-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	require.NoError(t, testApp.Services.Books.ToggleTag(
		context.Background(), owner, bookID, models.TagKoboSync,
	))

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	for _, e := range entries {
		ne, neOK := e["NewEntitlement"].(map[string]any)
		if !neOK {
			continue
		}
		if ent, entOK := ne["BookEntitlement"].(map[string]any); entOK {
			assert.NotEqual(t, bookID.String(), ent["Id"],
				"disabled book must not still be offered as NewEntitlement")
		}
	}

	found := false
	for _, e := range entries {
		ce, ok := e["ChangedEntitlement"].(map[string]any)
		if !ok {
			continue
		}
		ent, ok := ce["BookEntitlement"].(map[string]any)
		if !ok {
			continue
		}
		if ent["RevisionId"] == bookID.String() {
			found = true
			assert.Equal(t, true, ent["IsRemoved"])
		}
	}
	assert.True(t, found, "disabled book must appear as a removal entitlement")
}

// TestKoboLibrarySync_ReenabledBook_NoRemoval: re-enabling clears the removal.
func TestKoboLibrarySync_ReenabledBook_NoRemoval(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-reenabled-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	require.NoError(t, testApp.Services.Books.ToggleTag(
		context.Background(), owner, bookID, models.TagKoboSync,
	))
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), owner, bookID,
	))

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	for _, e := range entries {
		ce, ceOK := e["ChangedEntitlement"].(map[string]any)
		if !ceOK {
			continue
		}
		if ent, entOK := ce["BookEntitlement"].(map[string]any); entOK {
			assert.NotEqual(t, bookID.String(), ent["RevisionId"],
				"re-enabled book must not carry a stale removal entry")
		}
	}
}

// TestKoboMetadata_CrossUserProxied: another user's book is proxied upstream.
func TestKoboMetadata_CrossUserProxied(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	const userA = "kobo-meta-iso-user-a"
	_, bookID := setupKoboSyncBook(t, userA)

	rawTokenB := registerTestDevice(t, "kobo-meta-iso-user-b")

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawTokenB, "/v1/library/"+bookID.String()+"/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
