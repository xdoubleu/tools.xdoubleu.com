package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// registerDeviceReturningID registers a device for ownerID and returns both the
// raw token (for device-facing calls) and the device ID (the store key).
func registerDeviceReturningID(t *testing.T, ownerID string) (string, string) {
	t.Helper()
	device, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		context.Background(), ownerID, "Test Kobo", "",
	)
	require.NoError(t, err)
	return rawToken, device.ID
}

// --- Device-facing capture middleware ---

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
	// The sync manifest body must be captured (empty library serializes to
	// the JSON null/array the device receives).
	assert.NotEmpty(t, e.ResponseBody)
}

func TestKoboLogging_CapturesPutRequestBody(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	owner := "kobo-log-put-" + uuid.NewString()
	rawToken, bookID := setupKoboSyncBook(t, owner)

	// setupKoboSyncBook registers exactly one device for the owner; enable
	// logging on it (the raw token resolves to this device).
	devices, err := testApp.Services.Kobo.ListKoboDevices(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	deviceID := devices[0].ID
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })

	body := `{"ReadingState":{"CurrentBookmark":` +
		`{"ContentSourceProgressPercent":0.5,"Location":"chap-2"}}}`
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"),
		[]byte(body)))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var found bool
	for _, e := range testApp.Services.KoboLog.List(deviceID) {
		if e.Method == http.MethodPut &&
			strings.Contains(e.RequestBody, "ContentSourceProgressPercent") {
			assert.Contains(t, e.RequestBody, "chap-2")
			assert.Equal(t, http.StatusOK, e.Status)
			found = true
		}
	}
	assert.True(t, found, "PUT state request body must be captured")
}

// TestKoboLogging_BodyCaptureCapped verifies the captured request body is
// bounded (so debug logging can never buffer an unbounded body in memory)
// while the full body still reaches the handler.
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

	// A Location far larger than the 64 KiB capture cap.
	huge := strings.Repeat("x", 200*1024)
	body := `{"ReadingState":{"CurrentBookmark":` +
		`{"ContentSourceProgressPercent":0.5,"Location":"` + huge + `"}}}`
	resp, err := http.DefaultClient.Do(koboReq(t, http.MethodPut,
		koboURL(ts, rawToken, "/v1/library/"+bookID.String()+"/state"),
		[]byte(body)))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	const cap64KiB = 64 * 1024
	var found bool
	for _, e := range testApp.Services.KoboLog.List(deviceID) {
		if e.Method == http.MethodPut {
			found = true
			assert.LessOrEqual(t, len(e.RequestBody), cap64KiB,
				"captured request body must be capped")
			assert.NotEmpty(t, e.RequestBody)
		}
	}
	assert.True(t, found, "PUT must be captured")
}

// --- Connect RPCs ---

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

	// ListKoboDevices must reflect the in-memory logging flag.
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

	// Device owned by a different user than the authenticated one.
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

	// Generate one captured request.
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

func TestConnectClearKoboDeviceLogs_Empties(t *testing.T) {
	client := newBooksTestClient(t)
	ctx := context.Background()

	_, deviceID := registerDeviceReturningID(t, userID)
	testApp.Services.KoboLog.SetEnabled(deviceID, true)
	t.Cleanup(func() { testApp.Services.KoboLog.SetEnabled(deviceID, false) })
	testApp.Services.KoboLog.Append(deviceID, services.KoboLogEntry{
		Time: time.Now(), Method: "GET", Path: "/x", Query: "",
		RequestBody: "", Status: 200, ResponseBody: "",
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
