package kobogateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kobogateway"
)

const testOrigin = "https://tools.xdoubleu.com"

type stubUpdater struct {
	err    error
	origin string
	called bool
}

func (s *stubUpdater) SelfUpdate(_ context.Context, origin string) error {
	s.called = true
	s.origin = origin

	return s.err
}

func newTestServer(volumesRoot string, updater *stubUpdater) *kobogateway.Server {
	if updater == nil {
		updater = &stubUpdater{err: nil, origin: "", called: false}
	}

	return kobogateway.NewServer(kobogateway.Config{
		Port:           kobogateway.DefaultPort,
		AllowedOrigins: kobogateway.DefaultAllowedOrigins(),
		VolumesRoot:    volumesRoot,
		Release:        "testsha",
	}, updater)
}

func doRequest(
	handler http.Handler,
	method, path, origin, body string,
) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Host = "127.0.0.1:41132"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodPost && body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v))

	return v
}

func TestSecurityRejectsMissingOrigin(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(handler, http.MethodGet, "/status", "", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestSecurityRejectsUnknownOrigin(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(
		handler,
		http.MethodGet,
		"/status",
		"https://evil.example",
		"",
	)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSecurityRejectsForeignHost(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Host = "rebind.evil.example:41132"
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSecurityAllowsLocalhostHost(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Host = "localhost:41132"
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPreflight(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	req := httptest.NewRequest(http.MethodOptions, "/configure", nil)
	req.Host = "127.0.0.1:41132"
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, testOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(
		t,
		"true",
		rec.Header().Get("Access-Control-Allow-Private-Network"),
	)
}

func TestPreflightWithoutPrivateNetworkRequest(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	req := httptest.NewRequest(http.MethodOptions, "/status", nil)
	req.Host = "127.0.0.1:41132"
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestPostRequiresJSONContentType(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	req := httptest.NewRequest(
		http.MethodPost,
		"/configure",
		strings.NewReader(`{}`),
	)
	req.Host = "127.0.0.1:41132"
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStatusNoKobo(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(handler, http.MethodGet, "/status", testOrigin, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	status := decodeBody[kobogateway.StatusResponse](t, rec)
	assert.Equal(t, kobogateway.GatewayVersion, status.Version)
	assert.Equal(t, "testsha", status.Release)
	assert.Empty(t, status.Kobos)
}

func TestStatusWithKobo(t *testing.T) {
	root := t.TempDir()
	volumePath := makeKoboVolume(
		t,
		root,
		"KOBOeReader",
		sampleConf,
		"N418ABCD1234,4.38.21908",
	)
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(handler, http.MethodGet, "/status", testOrigin, "")

	status := decodeBody[kobogateway.StatusResponse](t, rec)
	assert.Equal(t, []kobogateway.Kobo{{
		VolumePath:      volumePath,
		Serial:          "N418ABCD1234",
		CurrentEndpoint: "https://storeapi.kobo.com",
	}}, status.Kobos)
}

func TestStatusUnreadableVolumesRoot(t *testing.T) {
	handler := newTestServer(
		filepath.Join(t.TempDir(), "missing"),
		nil,
	).Handler()

	rec := doRequest(handler, http.MethodGet, "/status", testOrigin, "")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestConfigure(t *testing.T) {
	root := t.TempDir()
	volumePath := makeKoboVolume(
		t,
		root,
		"KOBOeReader",
		sampleConf,
		"N418ABCD1234",
	)
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		`{"syncUrl":"https://tools.xdoubleu.com/books/kobo/TOKEN"}`,
	)

	assert.Equal(t, http.StatusOK, rec.Code)
	res := decodeBody[kobogateway.ConfigureResponse](t, rec)
	assert.Equal(t, "N418ABCD1234", res.Serial)
	assert.Equal(t, "https://storeapi.kobo.com", res.OriginalEndpoint)

	raw, err := os.ReadFile(
		filepath.Join(volumePath, ".kobo", "Kobo", "Kobo eReader.conf"),
	)
	require.NoError(t, err)
	conf := kobogateway.ParseConf(string(raw))
	assert.Equal(
		t,
		"https://tools.xdoubleu.com/books/kobo/TOKEN",
		conf.APIEndpoint(),
	)
}

func TestConfigureInvalidJSON(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBOeReader", sampleConf, "S1")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(handler, http.MethodPost, "/configure", testOrigin, "{")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestConfigureInvalidSyncURL(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBOeReader", sampleConf, "S1")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		`{"syncUrl":"not-a-url"}`,
	)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	res := decodeBody[kobogateway.ErrorResponse](t, rec)
	assert.Contains(t, res.Error, "syncUrl")
}

func TestConfigureNoKobo(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		`{"syncUrl":"https://tools.xdoubleu.com/books/kobo/TOKEN"}`,
	)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestConfigureMultipleKobosWithoutVolumePath(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBO1", sampleConf, "S1")
	makeKoboVolume(t, root, "KOBO2", sampleConf, "S2")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		`{"syncUrl":"https://tools.xdoubleu.com/books/kobo/TOKEN"}`,
	)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestConfigureExplicitVolumePath(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBO1", sampleConf, "S1")
	volumePath2 := makeKoboVolume(t, root, "KOBO2", sampleConf, "S2")
	handler := newTestServer(root, nil).Handler()

	body, err := json.Marshal(kobogateway.ConfigureRequest{
		SyncURL:    "https://tools.xdoubleu.com/books/kobo/TOKEN",
		VolumePath: volumePath2,
	})
	require.NoError(t, err)
	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		string(body),
	)

	assert.Equal(t, http.StatusOK, rec.Code)
	res := decodeBody[kobogateway.ConfigureResponse](t, rec)
	assert.Equal(t, "S2", res.Serial)
}

func TestConfigureUnknownVolumePath(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBO1", sampleConf, "S1")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/configure",
		testOrigin,
		`{"syncUrl":"https://x.example/kobo/T","volumePath":"/Volumes/NOPE"}`,
	)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRevert(t *testing.T) {
	root := t.TempDir()
	patched := strings.Replace(
		sampleConf,
		"https://storeapi.kobo.com",
		"https://tools.xdoubleu.com/books/kobo/TOKEN",
		1,
	)
	volumePath := makeKoboVolume(t, root, "KOBOeReader", patched, "S1")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/revert",
		testOrigin,
		`{"targetEndpoint":"https://storeapi.kobo.com"}`,
	)

	assert.Equal(t, http.StatusOK, rec.Code)
	res := decodeBody[kobogateway.RevertResponse](t, rec)
	assert.Equal(t, "S1", res.Serial)

	raw, err := os.ReadFile(
		filepath.Join(volumePath, ".kobo", "Kobo", "Kobo eReader.conf"),
	)
	require.NoError(t, err)
	assert.Equal(t, sampleConf, string(raw))
}

func TestRevertInvalidJSON(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(handler, http.MethodPost, "/revert", testOrigin, "{")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRevertNoKobo(t *testing.T) {
	handler := newTestServer(t.TempDir(), nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/revert",
		testOrigin,
		`{"targetEndpoint":"https://storeapi.kobo.com"}`,
	)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRevertInvalidTargetEndpoint(t *testing.T) {
	root := t.TempDir()
	makeKoboVolume(t, root, "KOBOeReader", sampleConf, "S1")
	handler := newTestServer(root, nil).Handler()

	rec := doRequest(
		handler,
		http.MethodPost,
		"/revert",
		testOrigin,
		`{"targetEndpoint":""}`,
	)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate(t *testing.T) {
	updater := &stubUpdater{err: nil, origin: "", called: false}
	gateway := newTestServer(t.TempDir(), updater)

	rec := doRequest(gateway.Handler(), http.MethodPost, "/update", testOrigin, `{}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	res := decodeBody[kobogateway.UpdateResponse](t, rec)
	assert.True(t, res.Updating)
	assert.True(t, updater.called)
	assert.Equal(t, testOrigin, updater.origin)

	select {
	case <-gateway.Restart():
	default:
		t.Fatal("expected a restart signal after a successful update")
	}
}

func TestUpdateFailure(t *testing.T) {
	updater := &stubUpdater{
		err:    errors.New("download failed"),
		origin: "",
		called: false,
	}
	gateway := newTestServer(t.TempDir(), updater)

	rec := doRequest(gateway.Handler(), http.MethodPost, "/update", testOrigin, `{}`)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	select {
	case <-gateway.Restart():
		t.Fatal("no restart signal expected after a failed update")
	default:
	}
}
