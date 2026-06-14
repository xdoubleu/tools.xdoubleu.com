package backlog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

// registerTestDevice registers a new Kobo device for ownerID and returns the
// raw token.  It exists as a helper because all Kobo route tests need a valid
// bearer token to authenticate.
func registerTestDevice(t *testing.T, ownerID string) string {
	t.Helper()
	_, rawToken, err := testApp.Services.Integrations.RegisterKoboDevice(
		context.Background(), ownerID, "Test Kobo", "",
	)
	require.NoError(t, err)
	return rawToken
}

// --- Proxy / upstream-merge tests ---

// TestKoboProxy_UnhandledPathProxied shows that a path we do not own is
// forwarded verbatim to the upstream Kobo store.
// FAILING before proxy implementation: ServeMux returns 404 for unknown paths.
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
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/UpgradeCheck"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	// After fix: upstream's 200; before fix: 404 from ServeMux.
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestKoboProxy_RequiresAuth shows that the catch-all proxy still enforces
// our bearer-token auth before forwarding to upstream.
func TestKoboProxy_RequiresAuth(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(upstream.Close)

	ts := httptest.NewServer(getRoutesWithKoboUpstream(t, upstream.URL))
	t.Cleanup(ts.Close)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet,
		koboURL(ts, "/v1/UpgradeCheck"), nil,
	)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")
	// Deliberately no Authorization header.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestKoboLibrarySync_MergesUpstreamItems shows that items returned by the
// upstream /v1/library/sync are preserved (additive merge).
// FAILING before merge implementation: upstream is never called, so only our
// books appear and the upstream item is absent.
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
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	// Upstream item must be present after the merge.
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

// TestKoboProxy_UpstreamDown_ReturnsBadGateway shows that the catch-all proxy
// returns 502 when the upstream Kobo store is unreachable.
func TestKoboProxy_UpstreamDown_ReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(
		getRoutesWithKoboUpstream(t, "http://127.0.0.1:0"),
	)
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-proxy-down-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/UpgradeCheck"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

// TestKoboLibrarySync_UpstreamNon200_FallsBackToOurBooks shows that a non-200
// from the upstream sync endpoint is gracefully degraded — our books still appear.
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
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	found := false
	for _, e := range entries {
		if ent, ok := e["BookEntitlement"].(map[string]any); ok {
			if ent["RevisionId"] == bookID.String() {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "our book must appear even when upstream returns non-200")
}

// TestKoboLibrarySync_ForwardsSyncToken shows that x-kobo-sync headers
// returned by the upstream are forwarded to the device.
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
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "continuation-abc", resp.Header.Get("x-kobo-sync"))
}

// TestKoboLibrarySync_OurBooksPreservedWhenUpstreamDown shows that our own
// books still appear in the sync response when the upstream is unreachable.
func TestKoboLibrarySync_OurBooksPreservedWhenUpstreamDown(t *testing.T) {
	owner := "kobo-upstream-down-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	// Use an invalid URL to simulate upstream being down.
	ts := httptest.NewServer(
		getRoutesWithKoboUpstream(t, "http://127.0.0.1:0"),
	)
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))

	found := false
	for _, e := range entries {
		if ent, ok := e["BookEntitlement"].(map[string]any); ok {
			if ent["RevisionId"] == bookID.String() {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "our book must still appear when upstream is down")
}

// koboURL builds a URL for the Kobo sync API on the given httptest server.
func koboURL(ts *httptest.Server, path string) string {
	return ts.URL + "/backlog/kobo" + path
}

// koboReq builds an HTTP request with the Kobo bearer token and HTTPS header set.
func koboReq(
	t *testing.T,
	method, url, rawToken string,
	body []byte,
) *http.Request {
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
	if rawToken != "" {
		req.Header.Set("Authorization", "Bearer "+rawToken)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	return req
}

// setupKoboSyncBook creates a book with a ready KEPUB in the fake objectstore,
// enables kobo-sync, and returns (rawToken, bookID).
// ownerID must be unique per test run to avoid DB accumulation across runs.
func setupKoboSyncBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	_, bookID := uploadFileForOwner(t, ownerID, models.FileFormatEPUB)
	// EnsureKEPUB converts + stores in the fake objectstore so PresignGet works.
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

// setupKoboPDFSyncBook creates a PDF-only book, tags it with kobo-format-pdf and
// kobo-sync, and returns (rawToken, bookID). Because the user wants raw-PDF sync
// there is no KEPUB row — the PDF itself is served to the device.
func setupKoboPDFSyncBook(t *testing.T, ownerID string) (string, uuid.UUID) {
	t.Helper()
	_, bookID := uploadFileForOwner(t, ownerID, models.FileFormatPDF)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), ownerID, bookID,
	))
	// Tag the book to serve the raw PDF to the Kobo.
	err := testApp.Repositories.Books.UpdateTags(
		context.Background(), ownerID, bookID,
		[]string{models.TagKoboSync, models.TagKoboFormatPDF},
	)
	require.NoError(t, err)
	rawToken := registerTestDevice(t, ownerID)
	return rawToken, bookID
}

// --- Auth / HTTPS gate tests ---

func TestKoboInit_MissingToken(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		koboURL(ts, "/v1/initialization"), nil,
	)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestKoboInit_InvalidToken(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	resp, err := http.DefaultClient.Do(
		koboReq(
			t,
			http.MethodPost,
			koboURL(ts, "/v1/initialization"),
			"bad-token-xyz",
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
		koboURL(ts, "/v1/initialization"), nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+rawToken)
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
		koboReq(t, http.MethodPost, koboURL(ts, "/v1/initialization"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "TokenList")
	assert.Contains(t, body, "Settings")
}

// --- Library sync ---

func TestKoboLibrarySync_EmptyLibrary(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-sync-empty-user")

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
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

	// Upload EPUB; enable kobo-sync but do NOT insert a ready KEPUB row
	// (simulates a book still converting).
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), owner, bookID,
	))

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	// Book with no ready KEPUB must not appear in the sync response.
	assert.Empty(t, entries)
}

func TestKoboLibrarySync_ReadyKEPUBIncluded(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-ready-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	entitlement, ok := entries[0]["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bookID.String(), entitlement["RevisionId"])

	dlUrls, ok := entries[0]["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KEPUB", dl["Format"])
	dlURL, ok := dl["Url"].(string)
	require.True(t, ok)
	assert.Contains(t, dlURL, bookID.String()+"/file")
}

// --- Cross-user isolation ---

func TestKoboLibrarySync_UserACannotSeeUserBBooks(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	// User B has a kobo-sync book with a ready KEPUB.
	userB := "kobo-iso-b-" + uuid.NewString()
	_, _ = setupKoboSyncBook(t, userB)

	// User A's token should return an empty library (unique ID — no prior books).
	rawTokenA := registerTestDevice(t, "kobo-iso-a-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawTokenA, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	assert.Empty(t, entries, "user A must not see user B's books")
}

// --- Invalid revision IDs ---

func TestKoboFile_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-file-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/not-a-uuid/file"), rawToken, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboGetState_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-gstate-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/not-a-uuid/state"), rawToken, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKoboPutState_InvalidRevisionID(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken := registerTestDevice(t, "kobo-pstate-badid-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, "/v1/library/not-a-uuid/state"), rawToken, []byte(`{}`)))
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
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"),
		rawToken, []byte(`not-json`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- File download ---

func TestKoboFile_Download_Redirect(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-file-dl-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	// Don't follow redirects so we can inspect the 302.
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req := koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/"+bookID.String()+"/file"), rawToken, nil)
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

	// User B's token trying to fetch user A's book.
	rawTokenB := registerTestDevice(t, "kobo-file-iso-user-b")

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/"+bookID.String()+"/file"), rawTokenB, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- Reading state round-trip ---

func TestKoboState_GetNoState(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-get-nostate-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"), rawToken, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)
}

func TestKoboState_PutThenGetRoundTrip(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	const owner = "kobo-state-roundtrip-user"
	rawToken, bookID := setupKoboSyncBook(t, owner)

	location := "chapter-3"
	body, err := json.Marshal(map[string]any{
		"ReadingState": map[string]any{
			"CurrentBookmark": map[string]any{
				"ContentSourceProgressPercent": 0.42,
				"Location":                     location,
			},
			"StatusInfo": map[string]any{"Status": "ReadyToRead"},
		},
	})
	require.NoError(t, err)

	// PUT state
	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"), rawToken, body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(t, http.StatusOK, putResp.StatusCode)

	// GET state — should reflect the written value.
	getResp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"), rawToken, nil))
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.42, bm["ContentSourceProgressPercent"], 0.01)
	assert.Equal(t, location, bm["Location"])
}

// TestKoboLibrarySync_PDFFormat_ServesPDF verifies that when a book has the
// kobo-format-pdf tag the sync manifest advertises "PDF" format and the correct
// content type, not "KEPUB".
func TestKoboLibrarySync_PDFFormat_ServesPDF(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-pdf-sync-" + uuid.NewString()
	rawToken, bookID := setupKoboPDFSyncBook(t, owner)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, "/v1/library/sync"), rawToken, nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	meta, ok := entries[0]["BookMetadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "application/pdf", meta["ContentType"])

	dlUrls, ok := entries[0]["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "PDF", dl["Format"])
	dlURL, ok := dl["Url"].(string)
	require.True(t, ok)
	assert.Contains(t, dlURL, bookID.String()+"/file")
}

// TestKoboFile_PDFFormat_RedirectsToPDF verifies that the file endpoint
// serves the PDF (not KEPUB) when the book has the kobo-format-pdf tag.
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
		koboURL(ts, "/v1/library/"+bookID.String()+"/file"), rawToken, nil)
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

	// Write state for user A.
	rawTokenA := registerTestDevice(t, userA)
	body, err := json.Marshal(map[string]any{
		"ReadingState": map[string]any{
			"CurrentBookmark": map[string]any{
				"ContentSourceProgressPercent": 0.9,
			},
			"StatusInfo": map[string]any{"Status": "ReadyToRead"},
		},
	})
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"), rawTokenA, body))
	require.NoError(t, err)
	putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	// User B token GETs the same book ID — must get a zero state, not user A's.
	rawTokenB := registerTestDevice(t, "kobo-state-iso-user-b")

	getResp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, "/v1/library/"+bookID.String()+"/state"), rawTokenB, nil))
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	// User B has no reading state for this book → percent must be 0.
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)
}
