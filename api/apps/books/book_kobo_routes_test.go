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
)

// registerTestDevice registers a new Kobo device for ownerID and returns the
// raw token. It exists as a helper because all Kobo route tests need a valid
// token to authenticate (embedded in the URL path, not a Bearer header).
func registerTestDevice(t *testing.T, ownerID string) string {
	t.Helper()
	_, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		context.Background(), ownerID, "Test Kobo", "",
	)
	require.NoError(t, err)
	return rawToken
}

// --- Proxy / upstream-merge tests ---

// TestKoboProxy_UnhandledPathProxied shows that a path we do not own is
// forwarded verbatim to the upstream Kobo store.
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

// TestKoboProxy_TokenStrippedFromUpstreamPath verifies that when the catch-all
// proxy forwards a request to the upstream Kobo store, the token segment is
// stripped so the upstream receives a clean /v1/… path, not /{token}/v1/….
// This is the regression test for the "sync failed / /auth/device 401" bug:
// previously the proxy forwarded the token to storeapi.kobo.com, which
// rejected the malformed path with a 401, causing "sync failed" on device.
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

	// The device requests: /{prefix}/kobo/{token}/v1/auth/device
	// The upstream must receive: /v1/auth/device (token stripped).
	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/auth/device"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/v1/auth/device", capturedPath,
		"upstream must receive /v1/auth/device without the token segment")
}

// TestKoboProxy_InvalidToken_Returns401 shows that the catch-all proxy still
// enforces our token auth before forwarding to upstream.
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

// TestKoboLibrarySync_MergesUpstreamItems shows that items returned by the
// upstream /v1/library/sync are preserved (additive merge).
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
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/UpgradeCheck"), nil),
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
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
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
			if ent["RevisionId"] == bookID.String() {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "our book must still appear when upstream is down")
}

// TestKoboLibrarySync_EmptyLibraryAndUpstreamDown is the regression test for
// the "stuck at Checking for updates…" hang: with zero kobo-sync books AND an
// unreachable upstream, append(nil, ...zero items) stays nil, which encodes as
// JSON null instead of []. The Kobo firmware expects an array and hangs on
// null. Body is checked as raw bytes because decoding into a slice silently
// turns null back into an empty slice, masking the bug.
func TestKoboLibrarySync_EmptyLibraryAndUpstreamDown(t *testing.T) {
	rawToken := registerTestDevice(t, "kobo-empty-and-down-"+uuid.NewString())

	// Use an invalid URL to simulate upstream being down.
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

// koboURL builds a URL for the Kobo sync API on the given httptest server.
// The rawToken is embedded in the path, matching the real device URL shape:
// the device sets api_endpoint = <server>/books/kobo/<rawToken> and appends
// store protocol paths (e.g. /v1/library/sync) to form the full request URL.
func koboURL(ts *httptest.Server, rawToken, path string) string {
	return ts.URL + "/books/kobo/" + rawToken + path
}

// koboReq builds an HTTP request with the HTTPS forwarded-proto header set.
// The Kobo token lives in the URL path (via koboURL), not in an auth header.
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
		true, // has kobo-sync tag
	)
	require.NoError(t, err)
	rawToken := registerTestDevice(t, ownerID)
	return rawToken, bookID
}

// --- Auth / HTTPS gate tests ---

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

// --- Library sync ---

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

	// Upload EPUB; enable kobo-sync but do NOT insert a ready KEPUB row
	// (simulates a book still converting).
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
	// Book with no ready KEPUB must not appear in the sync response.
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

	// Each of our entries must be wrapped in the NewEntitlement discriminator key.
	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok, "entry must be wrapped under NewEntitlement")

	entitlement, ok := ne["BookEntitlement"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bookID.String(), entitlement["RevisionId"])

	meta, ok := ne["BookMetadata"].(map[string]any)
	require.True(t, ok)
	dlUrls, ok := meta["DownloadUrls"].([]any)
	require.True(t, ok)
	require.Len(t, dlUrls, 1)
	dl, ok := dlUrls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KEPUB", dl["Format"])
	// Platform must be "Generic" so the device's DownloadUrlFilter=Generic,Android
	// header accepts the entry — "Desktop" is not in that list and is silently
	// discarded by the firmware, resulting in books never appearing on the device.
	assert.Equal(t, "Generic", dl["Platform"])
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
		koboReq(t, http.MethodGet, koboURL(ts, rawTokenA, "/v1/library/sync"), nil),
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

	// User B's token trying to fetch user A's book.
	rawTokenB := registerTestDevice(t, "kobo-file-iso-user-b")

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawTokenB, "/v1/library/"+bookID.String()+"/file"), nil))
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
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var state map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&state))
	bm, ok := state["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)

	// LastModified must be the epoch, not time.Now(). Returning "now" would
	// make the server always appear newer than the device, causing the firmware
	// to overwrite local progress with 0% and never push via PUT …/state.
	assert.Equal(t, "1970-01-01T00:00:00Z", state["LastModified"],
		"no-state LastModified must be epoch so device wins conflict and pushes")
	si, ok := state["StatusInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1970-01-01T00:00:00Z", si["LastModified"])
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
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(t, http.StatusOK, putResp.StatusCode)

	// GET state — should reflect the written value.
	getResp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), nil))
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

// TestKoboLibrarySync_ReadingStateIncluded verifies that every entry in the
// library sync manifest carries a non-nil ReadingState block, which is
// required for the Kobo firmware to participate in reading-state sync.
// Without it the device never issues PUT .../state and progress is never saved.
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
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001,
		"new book with no progress should advertise 0.0")

	// LastModified must be the epoch. Returning time.Now() would make the
	// server appear newer than the device on every sync, causing it to pull
	// 0% and never push progress via PUT …/state.
	assert.Equal(t, "1970-01-01T00:00:00Z", rs["LastModified"],
		"no-state ReadingState.LastModified must be epoch so device pushes progress")
}

// TestKoboLibrarySync_ReadingStateReflectsProgress verifies that after the
// device pushes progress via PUT .../state, the library sync manifest echoes
// that progress in the ReadingState block so other devices pick it up.
func TestKoboLibrarySync_ReadingStateReflectsProgress(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-sync-state-progress-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)
	location := "epubcfi(/6/4[chap01]!/4/2/1:0)"

	// Push progress as the device would.
	putBody, err := json.Marshal(map[string]any{
		"ReadingState": map[string]any{
			"CurrentBookmark": map[string]any{
				"ContentSourceProgressPercent": 0.65,
				"Location":                     location,
			},
		},
	})
	require.NoError(t, err)

	putResp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"), putBody))
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	// Sync — the manifest entry must now reflect the saved progress.
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
	assert.InDelta(t, 0.65, bm["ContentSourceProgressPercent"], 0.01)
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
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	// Each of our entries must be wrapped in the NewEntitlement discriminator key.
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
		koboURL(ts, rawTokenA, "/v1/library/"+bookID.String()+"/state"), body))
	require.NoError(t, err)
	putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	// User B token GETs the same book ID — must get a zero state, not user A's.
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
	// User B has no reading state for this book → percent must be 0.
	assert.InDelta(t, 0.0, bm["ContentSourceProgressPercent"], 0.001)
}

// --- Metadata endpoint tests ---

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

// TestKoboMetadata_UnknownBookProxiedUpstream verifies that a valid UUID for a
// book the user does not own (no kobo-sync row) is forwarded to the upstream
// Kobo store rather than returning a local 4xx.
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
	// Upstream responded 200, so we must relay it (not 400/404 locally).
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestKoboLibrarySync_EntitlementStableAcrossSyncs is the regression test for
// the "books briefly disappear then reappear" flicker: it syncs the same
// library twice and asserts the entitlement timestamps are identical. Before
// the fix, Created/PurchasedDate/ActivePeriod.From were time.Now() — different
// on every request — causing the Kobo firmware to tear down and recreate the
// entitlement on each sync.
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
}

// TestKoboLibrarySync_EntitlementTimestampIsEnableTime asserts that the
// Created/PurchasedDate/ActivePeriod.From timestamps in the sync manifest
// reflect the moment kobo-sync was enabled, not the request time.
func TestKoboLibrarySync_EntitlementTimestampIsEnableTime(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-enable-ts-" + uuid.NewString()

	// Truncate to seconds: the Created field is encoded as RFC3339 (second
	// precision), so sub-second timestamps would cause spurious failures when
	// the enable time straddles a second boundary.
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

	// The timestamp must fall within the window when we called EnableKoboSync,
	// not be equal to the request time (which would be later).
	assert.True(t, !ts2.Before(before) && !ts2.After(after),
		"Created must equal the kobo-sync enable time, got %s (window: %s–%s)",
		ts2, before, after)
}

// TestKoboMetadata_CrossUserProxied verifies that user B requesting user A's
// book ID is proxied upstream (not served from our DB).
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

	// User B's token requesting user A's book id.
	rawTokenB := registerTestDevice(t, "kobo-meta-iso-user-b")

	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboURL(ts, rawTokenB, "/v1/library/"+bookID.String()+"/metadata"), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	// Not in user B's kobo-sync list → proxied upstream, which returned 404.
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
