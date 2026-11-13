package spa_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/spa"
)

const indexHTML = `<!doctype html><html><head>` +
	`<title>tools.xdoubleu.com</title></head>` +
	`<body><div id="root"></div></body></html>`

const swJS = `(function(){})(self, __OFFLINE_ENABLED__, ["assets/app.js"]);`

func writeBuild(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":             indexHTML,
		"sw.js":                  swJS,
		"assets/app-abc123.js":   strings.Repeat("console.log('hi');", 50),
		"assets/pic-abc123.png":  "png-bytes",
		"icon.svg":               "<svg/>",
		"manifest.webmanifest":   `{"name":"tools"}`,
		"downloads/gateway.zip":  "zip-bytes",
		"downloads/big.tar.url":  "https://github.com/o/r/releases/download/t/big.tar\n",
		"downloads/bad.tar.url":  "https://evil.example/x",
		"assets/data.json":       `{"a":1}`,
		"assets/nested/chunk.js": "chunk",
	}
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}
	return dir
}

type logsRecorder struct {
	calls int
	body  string
}

func (l *logsRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.calls++
	b, _ := io.ReadAll(r.Body)
	l.body = string(b)
	w.WriteHeader(http.StatusNoContent)
}

func newHandler(t *testing.T, mutate func(*spa.Config)) (http.Handler, *logsRecorder) {
	t.Helper()
	logs := &logsRecorder{calls: 0, body: ""}
	cfg := spa.Config{
		Dir:                writeBuild(t),
		APIURL:             "https://tools.example/api",
		Release:            "abc123",
		KoboGatewayRelease: "def456",
		SentryDSN:          "https://key@sentry.example/1",
		PostHogKey:         "phc_key",
		PostHogHost:        "https://eu.i.posthog.com",
		OfflineDisabled:    false,
		APIHosts:           []string{"watchparty.example"},
		Logs:               logs,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	h, err := spa.New(cfg, slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("api:" + r.URL.Path))
	})
	return h.Wrap(api), logs
}

func get(
	t *testing.T,
	h http.Handler,
	target string,
	header ...string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestNew_FailsWithoutABuild(t *testing.T) {
	_, err := spa.New(
		spa.Config{Dir: t.TempDir()}, //nolint:exhaustruct // only Dir matters here
		slog.New(slog.DiscardHandler),
	)

	require.Error(t, err)
}

func TestNew_FailsWithoutAServiceWorker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexHTML), 0o600),
	)

	_, err := spa.New(
		spa.Config{Dir: dir}, //nolint:exhaustruct // only Dir matters here
		slog.New(slog.DiscardHandler),
	)

	require.Error(t, err)
}

func TestRouting_APIOwnedPathsReachTheAPI(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, path := range []string{
		"/api", "/api/version", "/api/books/x", "/.well-known/openid-configuration",
		"/health", "/metrics",
	} {
		rec := get(t, h, path)

		assert.Equal(t, "api:"+path, rec.Body.String(), path)
	}
}

func TestRouting_CustomDomainsReachTheAPI(t *testing.T) {
	h, _ := newHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "http://watchparty.example:8000/room", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, "api:/room", rec.Body.String())
}

func TestIndex_ServedForAppPaths(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, path := range []string{"/", "/books/library", "/books/7/read", "/trains/a.b"} {
		rec := get(t, h, path)

		assert.Equal(t, http.StatusOK, rec.Code, path)
		assert.Equal(t, indexHTML, rec.Body.String(), path)
		assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	}
}

func TestIndex_SecurityHeadersAndStaticCSP(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/")

	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, rec.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))

	csp := rec.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self' https://eu-assets.i.posthog.com")
	assert.NotContains(t, csp, "unsafe-eval")
	assert.NotContains(t, csp, "nonce")
	// Readium's blob: frames and the pdf.js worker.
	assert.Contains(t, csp, "frame-src 'self' blob:")
	assert.Contains(t, csp, "worker-src 'self' blob:")
	assert.Contains(t, csp, "style-src 'self' 'unsafe-inline' blob:")
	assert.Contains(t, csp, "font-src 'self' blob:")
	assert.Contains(t, csp, "frame-ancestors 'self'")
	// connect-src: R2 uploads, Sentry, PostHog, the api and the kobo gateway.
	_, connect, _ := strings.Cut(csp, "connect-src")
	connect, _, _ = strings.Cut(connect, ";")
	for _, want := range []string{
		"'self'", "*.sentry.io", "https://*.r2.cloudflarestorage.com",
		"https://127.0.0.1:41132", "https://tools.example",
		"https://eu.i.posthog.com", "https://eu-assets.i.posthog.com",
	} {
		assert.Contains(t, connect, want)
	}
}

func TestIndex_NoPostHogLeavesItOutOfThePolicy(t *testing.T) {
	h, _ := newHandler(t, func(c *spa.Config) { c.PostHogHost = ""; c.APIURL = "" })

	csp := get(t, h, "/").Header().Get("Content-Security-Policy")

	assert.Contains(t, csp, "script-src 'self';")
	assert.NotContains(t, csp, "posthog")
}

func TestIndex_SharePagesGetATitleAndNoIndex(t *testing.T) {
	h, _ := newHandler(t, nil)

	for path, title := range map[string]string{
		"/dashboard/games/tok":           "Shared games",
		"/dashboard/games/tok/library":   "Shared library",
		"/dashboard/games/tok/730":       "Shared game",
		"/dashboard/reading/tok":         "Shared reading dashboard",
		"/dashboard/reading/tok/library": "Shared library",
	} {
		body := get(t, h, path).Body.String()

		assert.Contains(t, body, "<title>"+title+"</title>", path)
		assert.Contains(t, body, `<meta name="robots" content="noindex, nofollow" />`, path)
		assert.NotContains(t, body, "<title>tools.xdoubleu.com</title>", path)
	}
}

func TestIndex_OwnerDashboardsKeepTheDefaultTitle(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, path := range []string{
		"/dashboard", "/dashboard/games", "/dashboard/reading", "/books/library",
		"/dashboard/reading/tok/extra", "/dashboard/other/tok",
	} {
		body := get(t, h, path).Body.String()

		assert.Equal(t, indexHTML, body, path)
	}
}

func TestStatic_HashedAssetsAreImmutable(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/assets/pic-abc123.png")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "png-bytes", rec.Body.String())
	assert.Equal(
		t,
		"public, max-age=31536000, immutable",
		rec.Header().Get("Cache-Control"),
	)
}

func TestStatic_OtherFilesRevalidate(t *testing.T) {
	h, _ := newHandler(t, nil)

	for path, contentType := range map[string]string{
		"/icon.svg":              "image/svg+xml",
		"/manifest.webmanifest":  "application/manifest+json",
		"/downloads/gateway.zip": "application/zip",
	} {
		rec := get(t, h, path)

		assert.Equal(t, http.StatusOK, rec.Code, path)
		assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"), path)
		assert.Contains(t, rec.Header().Get("Content-Type"), contentType, path)
	}
}

func TestStatic_GzipsTextWhenAccepted(t *testing.T) {
	h, _ := newHandler(t, nil)
	plain := get(t, h, "/assets/app-abc123.js")

	rec := get(t, h, "/assets/app-abc123.js", "Accept-Encoding", "gzip, br")

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "javascript")
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	got, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, plain.Body.String(), string(got))
	assert.Empty(t, plain.Header().Get("Content-Encoding"))
	assert.Less(t, rec.Body.Len(), plain.Body.Len())

	// The cached copy answers the next request.
	again := get(t, h, "/assets/app-abc123.js", "Accept-Encoding", "gzip")
	assert.Equal(t, rec.Body.Bytes(), again.Body.Bytes())
}

func TestStatic_DoesNotGzipBinaries(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/assets/pic-abc123.png", "Accept-Encoding", "gzip")

	assert.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestStatic_MissingAssetsAre404NotTheShell(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, path := range []string{
		"/assets/gone-123.js", "/assets/gone", "/missing.css", "/pic.png", "/assets/nested",
	} {
		rec := get(t, h, path)

		assert.Equal(t, http.StatusNotFound, rec.Code, path)
	}
}

func TestStatic_RefusesToEscapeTheBuildDirectory(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/assets/../../etc/passwd")

	assert.NotContains(t, rec.Body.String(), "root:")
	assert.Equal(t, indexHTML, rec.Body.String())
}

func TestStatic_ShellAndWorkerFilesAreNotServedRaw(t *testing.T) {
	h, _ := newHandler(t, nil)

	assert.Equal(t, indexHTML, get(t, h, "/index.html").Body.String())
	assert.NotContains(t, get(t, h, "/sw.js").Body.String(), "__OFFLINE_ENABLED__")
}

func TestStatic_OnlyGetAndHead(t *testing.T) {
	h, _ := newHandler(t, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/books/library", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "GET, HEAD", rec.Header().Get("Allow"))
}

func TestStatic_HeadHasNoBody(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, path := range []string{"/", "/env.js", "/sw.js", "/release"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, path, nil))

		assert.Equal(t, http.StatusOK, rec.Code, path)
		assert.Empty(t, rec.Body.String(), path)
	}
}

func TestEnvJS_CarriesRuntimeConfigUncached(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/env.js")

	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(
		t,
		"application/javascript; charset=utf-8",
		rec.Header().Get("Content-Type"),
	)
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, "window.__ENV__="))
	for _, want := range []string{
		`"API_URL":"https://tools.example/api"`, `"RELEASE":"abc123"`,
		`"KOBO_GATEWAY_RELEASE":"def456"`, `"SENTRY_DSN_WEB":"https://key@sentry.example/1"`,
		`"POSTHOG_KEY":"phc_key"`, `"POSTHOG_HOST":"https://eu.i.posthog.com"`,
	} {
		assert.Contains(t, body, want)
	}
}

func TestServiceWorker_EnabledByDefault(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/sw.js")

	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, `(function(){})(self, true, ["assets/app.js"]);`, rec.Body.String())
}

func TestServiceWorker_KillSwitch(t *testing.T) {
	h, _ := newHandler(t, func(c *spa.Config) { c.OfflineDisabled = true })

	assert.Contains(t, get(t, h, "/sw.js").Body.String(), "(self, false,")
}

func TestRelease_ReportsTheDeployedRelease(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/release")

	assert.Equal(t, `{"release":"abc123"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func post(
	t *testing.T,
	h http.Handler,
	target, body string,
	header ...string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:1234"
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLogs_ForwardsSameOriginBatches(t *testing.T) {
	h, logs := newHandler(t, nil)

	rec := post(t, h, "/logs", `{"entries":[]}`, "Sec-Fetch-Site", "same-origin")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, logs.calls)
	assert.Equal(t, `{"entries":[]}`, logs.body)
}

func TestLogs_AllowsClientsWithoutFetchMetadata(t *testing.T) {
	h, logs := newHandler(t, nil)

	post(t, h, "/logs", `{}`)

	assert.Equal(t, 1, logs.calls)
}

func TestLogs_RejectsCrossSite(t *testing.T) {
	h, logs := newHandler(t, nil)

	rec := post(t, h, "/logs", `{}`, "Sec-Fetch-Site", "cross-site")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Zero(t, logs.calls)
}

func TestLogs_RejectsOversizeBodies(t *testing.T) {
	h, logs := newHandler(t, nil)

	rec := post(t, h, "/logs", strings.Repeat("x", 64*1024+1))

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Zero(t, logs.calls)
}

func TestLogs_RateLimitsPerIP(t *testing.T) {
	h, logs := newHandler(t, nil)

	var limited int
	for range 100 {
		if post(t, h, "/logs", `{}`).Code == http.StatusTooManyRequests {
			limited++
		}
	}

	assert.Positive(t, limited)
	assert.Less(t, logs.calls, 100)
}

func TestLogs_OnlyPost(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/logs")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "POST", rec.Header().Get("Allow"))
}

func TestLogs_DroppedWithoutAHandler(t *testing.T) {
	h, _ := newHandler(t, func(c *spa.Config) { c.Logs = nil })

	rec := post(t, h, "/logs", `{}`)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestWebVitals_RecordsKnownMetrics(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := post(t, h, "/web-vitals", `{"name":"TTFB","value":120}`)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestWebVitals_AcceptsButDropsUnknownMetrics(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, body := range []string{
		`{"name":"BOGUS","value":1}`, `{"name":"LCP","value":-1}`,
	} {
		rec := post(t, h, "/web-vitals", body)

		assert.Equal(t, http.StatusAccepted, rec.Code, body)
	}
}

func TestWebVitals_RejectsMalformedBodies(t *testing.T) {
	h, _ := newHandler(t, nil)

	for _, body := range []string{
		`nope`, `{"name":"TTFB"}`, `{"value":1}`, `{"name":1,"value":1}`,
	} {
		rec := post(t, h, "/web-vitals", body)

		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

func TestWebVitals_RejectsCrossSiteAndOversize(t *testing.T) {
	h, _ := newHandler(t, nil)

	cross := post(
		t,
		h,
		"/web-vitals",
		`{"name":"TTFB","value":1}`,
		"Sec-Fetch-Site",
		"cross-site",
	)
	big := post(t, h, "/web-vitals", strings.Repeat("x", 2048))

	assert.Equal(t, http.StatusForbidden, cross.Code)
	assert.Equal(t, http.StatusRequestEntityTooLarge, big.Code)
}

func TestWebVitals_OnlyPost(t *testing.T) {
	h, _ := newHandler(t, nil)

	rec := get(t, h, "/web-vitals")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestDownloadPointerRedirects(t *testing.T) {
	h, _ := newHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/downloads/big.tar", nil))
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://github.com/o/r/releases/download/t/big.tar",
		rec.Header().Get("Location"))

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/downloads/bad.tar", nil))
	require.NotEqual(t, http.StatusFound, rec.Code)
}
