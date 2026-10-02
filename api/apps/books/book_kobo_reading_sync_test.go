package books_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// koboStateURL is the device-shaped state endpoint for bookID.
func koboStateURL(ts *httptest.Server, rawToken string, bookID uuid.UUID) string {
	return koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state")
}

// koboPutState PUTs body and returns the decoded response.
func koboPutState(
	t *testing.T,
	ts *httptest.Server,
	rawToken string,
	bookID uuid.UUID,
	body any,
) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboStateURL(ts, rawToken, bookID), raw))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

// koboGetState GETs the device-facing reading state.
func koboGetState(
	t *testing.T,
	ts *httptest.Server,
	rawToken string,
	bookID uuid.UUID,
) map[string]any {
	t.Helper()
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodGet,
		koboStateURL(ts, rawToken, bookID), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

// koboSync runs a library sync and returns its entries.
func koboSync(t *testing.T, ts *httptest.Server, rawToken string) []map[string]any {
	t.Helper()
	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	return entries
}

// changedReadingStates returns the ReadingState of every ChangedReadingState.
func changedReadingStates(entries []map[string]any) []map[string]any {
	var out []map[string]any
	for _, e := range entries {
		crs, ok := e["ChangedReadingState"].(map[string]any)
		if !ok {
			continue
		}
		rs, _ := crs["ReadingState"].(map[string]any)
		out = append(out, rs)
	}
	return out
}

// kepubLocationPut is a real-device PUT body for a reading position.
func kepubLocationPut(percent int, value, status, lastModified string) any {
	return map[string]any{
		"ReadingStates": []map[string]any{{
			"EntitlementId": "ignored",
			"CurrentBookmark": map[string]any{
				"LastModified":                 lastModified,
				"ProgressPercent":              percent,
				"ContentSourceProgressPercent": 40,
				"Location": map[string]any{
					"Source": "OEBPS/chapter03.xhtml",
					"Type":   "KoboSpan",
					"Value":  value,
				},
			},
			"StatusInfo": map[string]any{"Status": status},
		}},
	}
}

func TestKoboLibrarySync_ChangedReadingState_OncePerChange(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-changed-state-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	assert.Empty(t, changedReadingStates(koboSync(t, ts, rawToken)),
		"first delivery carries the state in NewEntitlement only")

	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(), owner, bookID, models.ReadingSourceWeb, 42, nil,
	))

	changed := changedReadingStates(koboSync(t, ts, rawToken))
	require.Len(t, changed, 1)
	assert.Equal(t, bookID.String(), changed[0]["EntitlementId"])
	assert.Equal(t, changed[0]["LastModified"], changed[0]["PriorityTimestamp"])
	bm, ok := changed[0]["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 42.0, bm["ProgressPercent"], 0.01)

	assert.Empty(t, changedReadingStates(koboSync(t, ts, rawToken)),
		"an unchanged state must not be re-sent")
}

func TestKoboLibrarySync_StateBeforeFirstSync_NoChangedReadingState(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-state-before-sync-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)
	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(), owner, bookID, models.ReadingSourceWeb, 20, nil,
	))

	entries := koboSync(t, ts, rawToken)
	assert.Empty(t, changedReadingStates(entries))
	require.Len(t, entries, 1)
	ne, ok := entries[0]["NewEntitlement"].(map[string]any)
	require.True(t, ok)
	rs, ok := ne["ReadingState"].(map[string]any)
	require.True(t, ok)
	bm, ok := rs["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 20.0, bm["ProgressPercent"], 0.01)
}

// TestKoboLibrarySync_DevicePut_ReachesOtherDeviceOnly: the writing device
// already has its own state; a second device gets it as ChangedReadingState.
func TestKoboLibrarySync_DevicePut_ReachesOtherDeviceOnly(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-two-devices-" + uuid.NewString()
	tokenA, bookID := setupKoboSyncBook(t, owner)
	tokenB := registerTestDevice(t, owner)

	koboSync(t, ts, tokenA)
	koboSync(t, ts, tokenB)

	koboPutState(t, ts, tokenA, bookID,
		kepubLocationPut(30, "kobo.12.3", "Reading", "2026-09-30T08:15:42Z"))

	assert.Empty(t, changedReadingStates(koboSync(t, ts, tokenA)),
		"the writing device must not get its own state echoed back")

	changed := changedReadingStates(koboSync(t, ts, tokenB))
	require.Len(t, changed, 1)
	bm, ok := changed[0]["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"Source": "OEBPS/chapter03.xhtml",
		"Type":   "KoboSpan",
		"Value":  "kobo.12.3",
	}, bm["Location"])

	assert.Empty(t, changedReadingStates(koboSync(t, ts, tokenB)))
}

// TestKoboLibrarySync_RegressedPut_KeepsPendingChange: a PUT dropped by the
// no-regress guard must not mark a newer server state as delivered.
func TestKoboLibrarySync_RegressedPut_KeepsPendingChange(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-regressed-put-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)
	koboSync(t, ts, rawToken)

	require.NoError(t, testApp.Services.Books.UpdateReadingProgress(
		context.Background(), owner, bookID, models.ReadingSourceWeb, 70, nil,
	))
	koboPutState(t, ts, rawToken, bookID,
		kepubLocationPut(10, "kobo.2.1", "Reading", "2026-09-30T08:15:42Z"))

	changed := changedReadingStates(koboSync(t, ts, rawToken))
	require.Len(t, changed, 1)
	bm, ok := changed[0]["CurrentBookmark"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 70.0, bm["ProgressPercent"], 0.01)
}
