package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

// registerDeviceReturningID returns a new device's raw token and ID.
func registerDeviceReturningID(t *testing.T, ownerID string) (string, string) {
	t.Helper()
	device, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		context.Background(), ownerID, "Test Kobo", "",
	)
	require.NoError(t, err)
	return rawToken, device.ID
}

func TestKoboLogging_DisabledCapturesNothing(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, deviceID := registerDeviceReturningID(t, "kobo-log-off-"+uuid.NewString())

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Empty(t, testApp.Services.KoboLog.List(deviceID),
		"nothing must be captured while logging is disabled")
}

func TestKoboLogging_CapturesSyncRequestAndResponse(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, deviceID := registerDeviceReturningID(
		t,
		"kobo-log-sync-"+uuid.NewString(),
	)
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	entries := testApp.Services.KoboLog.List(deviceID)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, http.MethodGet, e.Method)
	assert.True(t, strings.HasSuffix(e.Path, "/v1/library/sync"))
	assert.NotContains(t, e.Path, rawToken,
		"the device's live sync token must never appear in captured logs")
	assert.Equal(t, http.StatusOK, e.Status)
	assert.NotEmpty(t, e.ResponseBody)
}

func TestKoboLogging_CapturesPutRequestBody(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-log-put-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	devices, err := testApp.Services.Kobo.ListKoboDevices(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	deviceID := devices[0].ID
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	body := `{"ReadingStates":[{"CurrentBookmark":` +
		`{"ProgressPercent":50,"Location":"chap-2"}}]}`
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"),
		[]byte(body)))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The append happens after the response is written; poll.
	var entry services.KoboLogEntry
	require.Eventually(t, func() bool {
		for _, e := range testApp.Services.KoboLog.List(deviceID) {
			if e.Method == http.MethodPut &&
				strings.Contains(e.RequestBody, "ProgressPercent") {
				entry = e
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "PUT state request body must be captured")
	assert.Contains(t, entry.RequestBody, "chap-2")
	assert.Equal(t, http.StatusOK, entry.Status)
}

// TestKoboLogging_BodyCaptureCapped: capture is capped while the handler still
// gets the full body.
func TestKoboLogging_BodyCaptureCapped(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-log-cap-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	devices, err := testApp.Services.Kobo.ListKoboDevices(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	deviceID := devices[0].ID
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	huge := strings.Repeat("x", 200*1024)
	body := `{"ReadingStates":[{"CurrentBookmark":` +
		`{"ProgressPercent":50,"Location":"` + huge + `"}}]}`
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"),
		[]byte(body)))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The append happens after the response is written; poll.
	const cap64KiB = 64 * 1024
	var entry services.KoboLogEntry
	require.Eventually(t, func() bool {
		for _, e := range testApp.Services.KoboLog.List(deviceID) {
			if e.Method == http.MethodPut {
				entry = e
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "PUT must be captured")
	assert.LessOrEqual(t, len(entry.RequestBody), cap64KiB,
		"captured request body must be capped")
	assert.NotEmpty(t, entry.RequestBody)
}

func TestConnectSetKoboDeviceLogging_TogglesAndReflectsInList(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	_, deviceID := registerDeviceReturningID(t, userID)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	setReq := connect.NewRequest(&booksv1.SetKoboDeviceLoggingRequest{
		Id: deviceID, Enabled: true,
	})
	setReq.Header().Set("Cookie", accessToken.String())
	_, err := client.SetKoboDeviceLogging(ctx, setReq)
	require.NoError(t, err)
	assert.True(t, testApp.Services.KoboLog.IsEnabled(deviceID))

	listReq := connect.NewRequest(&booksv1.ListKoboDevicesRequest{})
	listReq.Header().Set("Cookie", accessToken.String())
	listResp, err := client.ListKoboDevices(ctx, listReq)
	require.NoError(t, err)
	var seen bool
	for _, d := range listResp.Msg.Devices {
		if d.Id == deviceID {
			assert.True(t, d.LoggingEnabled)
			seen = true
		}
	}
	assert.True(t, seen, "registered device must appear in the list")
}

func TestConnectSetKoboDeviceLogging_OtherUsersDeviceNotFound(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	_, deviceID := registerDeviceReturningID(t, "kobo-log-other-"+uuid.NewString())

	req := connect.NewRequest(&booksv1.SetKoboDeviceLoggingRequest{
		Id: deviceID, Enabled: true,
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.SetKoboDeviceLogging(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	assert.False(t, testApp.Services.KoboLog.IsEnabled(deviceID))
}

func TestConnectGetKoboDeviceLogs_ReturnsEntries(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	rawToken, deviceID := registerDeviceReturningID(t, userID)
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost,
			koboURL(ts, rawToken, "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	resp.Body.Close()

	req := connect.NewRequest(&booksv1.GetKoboDeviceLogsRequest{Id: deviceID})
	req.Header().Set("Cookie", accessToken.String())
	logsResp, err := client.GetKoboDeviceLogs(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, logsResp.Msg.Entries)
	assert.Equal(t, http.MethodPost, logsResp.Msg.Entries[0].Method)
	assert.NotEmpty(t, logsResp.Msg.Entries[0].ResponseBody)
}

func TestConnectGetKoboDeviceLogs_InvalidUTF8ResponseBody(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	_, deviceID := registerDeviceReturningID(t, userID)
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	// Invalid UTF-8, as captured from a binary/gzip upstream response.
	testApp.Services.KoboLog.Append(deviceID, services.KoboLogEntry{
		Time: time.Now(), Method: "GET", Path: "/x", Query: "",
		RequestBody: "", Status: 200,
		ResponseBody:    string([]byte{0xff, 0xfe, 0x00}),
		Notes:           "",
		RequestHeaders:  nil,
		ResponseHeaders: map[string]string{"x-kobo-sync": "continue"},
		UpstreamHeaders: map[string]string{
			"x-kobo-synctoken": string([]byte{0xff, 0xfe}),
		},
	})

	req := connect.NewRequest(&booksv1.GetKoboDeviceLogsRequest{Id: deviceID})
	req.Header().Set("Cookie", accessToken.String())
	logsResp, err := client.GetKoboDeviceLogs(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, logsResp.Msg.Entries)
	got := logsResp.Msg.Entries[0]
	assert.True(t, utf8.ValidString(got.ResponseBody))
	assert.Equal(t, "continue", got.ResponseHeaders["x-kobo-sync"])
	assert.True(t, utf8.ValidString(got.UpstreamHeaders["x-kobo-synctoken"]))
	assert.Empty(t, got.RequestHeaders)
}

func TestConnectClearKoboDeviceLogs_Empties(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	_, deviceID := registerDeviceReturningID(t, userID)
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })
	testApp.Services.KoboLog.Append(deviceID, services.KoboLogEntry{
		Time: time.Now(), Method: "GET", Path: "/x", Query: "",
		RequestBody: "", Status: 200, ResponseBody: "", Notes: "",
		RequestHeaders: nil, ResponseHeaders: nil, UpstreamHeaders: nil,
	})
	require.NotEmpty(t, testApp.Services.KoboLog.List(deviceID))

	req := connect.NewRequest(&booksv1.ClearKoboDeviceLogsRequest{Id: deviceID})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.ClearKoboDeviceLogs(ctx, req)
	require.NoError(t, err)

	assert.Empty(t, testApp.Services.KoboLog.List(deviceID))
	assert.True(t, testApp.Services.KoboLog.IsEnabled(deviceID),
		"clearing logs must not disable logging")
}

func TestConnectKoboLogging_InvalidDeviceID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	req := connect.NewRequest(&booksv1.GetKoboDeviceLogsRequest{Id: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.GetKoboDeviceLogs(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// enableUpstreamLogging registers a device on app and turns logging on.
func enableUpstreamLogging(
	t *testing.T,
	app *books.Books,
	owner string,
) (string, string) {
	t.Helper()
	device, rawToken, err := app.Services.Kobo.RegisterKoboDevice(
		context.Background(), owner, "Test Kobo", "",
	)
	require.NoError(t, err)
	app.Services.KoboLog.SetEnabled(device.ID, true)
	return rawToken, device.ID
}

// waitForLogEntry polls for the device's first entry on path; the append
// happens after the response is written.
func waitForLogEntry(
	t *testing.T,
	app *books.Books,
	deviceID, pathSuffix string,
) services.KoboLogEntry {
	t.Helper()
	var entry services.KoboLogEntry
	require.Eventually(t, func() bool {
		for _, e := range app.Services.KoboLog.List(deviceID) {
			if strings.HasSuffix(e.Path, pathSuffix) {
				entry = e
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "%s must be captured", pathSuffix)
	return entry
}

func assertNoCredentialHeaders(t *testing.T, e services.KoboLogEntry) {
	t.Helper()
	for _, m := range []map[string]string{
		e.RequestHeaders, e.ResponseHeaders, e.UpstreamHeaders,
	} {
		for k, v := range m {
			assert.NotContains(t, []string{"authorization", "cookie", "set-cookie"}, k)
			assert.NotContains(t, v, "secret")
		}
	}
}

func TestKoboLogging_SyncRecordsAllowlistedHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("x-kobo-sync", "continue")
			w.Header().Set("x-kobo-sync-mode", "delta")
			w.Header().Set("x-kobo-synctoken", "upstream-token")
			w.Header().Set("Set-Cookie", "session=secret")
			_, _ = w.Write([]byte("[]"))
		},
	))
	t.Cleanup(upstream.Close)

	app := newAppWithKoboUpstream(t, upstream.URL)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)

	rawToken, deviceID := enableUpstreamLogging(t, app, "kobo-log-hdr-"+uuid.NewString())

	req := koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/library/sync"), nil)
	req.Header.Set("x-kobo-synctoken", "device-token")
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "session=secret")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	e := waitForLogEntry(t, app, deviceID, "/v1/library/sync")
	assert.Equal(t,
		map[string]string{"x-kobo-synctoken": "device-token"}, e.RequestHeaders)
	assert.Equal(t, map[string]string{
		"x-kobo-sync":      "continue",
		"x-kobo-sync-mode": "delta",
		"x-kobo-synctoken": "upstream-token",
	}, e.UpstreamHeaders)
	assert.Equal(t, "continue", e.ResponseHeaders["x-kobo-sync"],
		"the response map must show what the device actually received")
	assertNoCredentialHeaders(t, e)
}

func TestKoboLogging_ProxyRecordsUpstreamHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Encoding", "identity")
			w.Header().Set("Set-Cookie", "session=secret")
			_, _ = w.Write([]byte(`"firmware-ok"`))
		},
	))
	t.Cleanup(upstream.Close)

	app := newAppWithKoboUpstream(t, upstream.URL)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)

	rawToken, deviceID := enableUpstreamLogging(t, app, "kobo-log-proxy-"+uuid.NewString())

	req := koboReq(t, http.MethodGet, koboURL(ts, rawToken, "/v1/UpgradeCheck"), nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	e := waitForLogEntry(t, app, deviceID, "/v1/UpgradeCheck")
	want := map[string]string{"content-encoding": "identity"}
	assert.Equal(t, want, e.UpstreamHeaders)
	assert.Equal(t, want, e.ResponseHeaders)
	assert.Empty(t, e.RequestHeaders)
	assertNoCredentialHeaders(t, e)
}
