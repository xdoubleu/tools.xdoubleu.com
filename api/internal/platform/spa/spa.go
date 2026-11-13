// Package spa serves the built web app (web/dist): hashed assets, the HTML
// shell with a static CSP, the runtime config script, and the service worker.
// It sits in front of the API handler and leaves API routes to it.
package spa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	indexFile = "index.html"
	swFile    = "sw.js"
	// offlineEnabledToken is the placeholder web/vite.config.ts leaves in the
	// service worker for the OFFLINE_DISABLED kill switch.
	//nolint:gosec // a placeholder, not a credential
	offlineEnabledToken = "__OFFLINE_ENABLED__"
	immutableCache      = "public, max-age=31536000, immutable"
	revalidateCache     = "no-cache"
	noStore             = "no-store"
	extJS               = ".js"
	extMJS              = ".mjs"
	extManifest         = ".webmanifest"
	maxLogsBodyBytes    = 64 * 1024
	maxVitalsBodyBytes  = 1024
	logsPerMinute       = 60
	vitalsPerMinute     = 600
)

// Config is what the shell and its runtime script need from the api config.
type Config struct {
	// Dir holds web's build output (index.html, assets/, sw.js, ...).
	Dir string
	// APIURL is the public API base the browser calls, e.g. https://host/api.
	APIURL             string
	Release            string
	KoboGatewayRelease string
	SentryDSN          string
	PostHogKey         string
	PostHogHost        string
	// OfflineDisabled serves a service worker that unregisters itself.
	OfflineDisabled bool
	// APIHosts are custom domains the api serves itself (e.g. watchparty).
	APIHosts []string
	// Logs receives POST /logs after the same-origin, size and rate checks.
	Logs http.Handler
}

// Handler serves the web app in front of the API.
type Handler struct {
	cfg      Config
	root     *os.Root
	logger   *slog.Logger
	index    []byte
	sw       []byte
	csp      string
	envJS    []byte
	apiHosts map[string]bool

	variantsMu sync.Mutex
	variants   map[string][]byte

	gzip   gzipCache
	logs   *limiter
	vitals *limiter
}

// New loads the build output; it fails when the build is missing, so a bad
// image doesn't boot into serving nothing.
func New(cfg Config, logger *slog.Logger) (*Handler, error) {
	// os.Root refuses any path that escapes the build directory.
	root, err := os.OpenRoot(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("open web build: %w", err)
	}
	index, err := root.ReadFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", indexFile, err)
	}
	sw, err := root.ReadFile(swFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", swFile, err)
	}
	enabled := "true"
	if cfg.OfflineDisabled {
		enabled = "false"
	}
	sw = bytes.ReplaceAll(sw, []byte(offlineEnabledToken), []byte(enabled))

	env, err := runtimeEnv(cfg)
	if err != nil {
		return nil, err
	}

	hosts := make(map[string]bool, len(cfg.APIHosts))
	for _, h := range cfg.APIHosts {
		hosts[h] = true
	}

	registerMimeTypes()

	return &Handler{
		cfg:        cfg,
		root:       root,
		logger:     logger,
		index:      index,
		sw:         sw,
		csp:        contentSecurityPolicy(cfg),
		envJS:      env,
		apiHosts:   hosts,
		variantsMu: sync.Mutex{},
		variants:   map[string][]byte{},
		gzip:       newGzipCache(),
		logs:       newLimiter(logsPerMinute),
		vitals:     newLimiter(vitalsPerMinute),
	}, nil
}

func registerMimeTypes() {
	for ext, typ := range map[string]string{
		extManifest: "application/manifest+json",
		extMJS:      "text/javascript; charset=utf-8",
		extJS:       "text/javascript; charset=utf-8",
	} {
		_ = mime.AddExtensionType(ext, typ)
	}
}

// runtimeEnv renders window.__ENV__, which web/lib/env.ts reads.
func runtimeEnv(cfg Config) ([]byte, error) {
	values, err := json.Marshal(map[string]string{
		"API_URL":              cfg.APIURL,
		"SENTRY_DSN_WEB":       cfg.SentryDSN,
		"RELEASE":              cfg.Release,
		"KOBO_GATEWAY_RELEASE": cfg.KoboGatewayRelease,
		"POSTHOG_KEY":          cfg.PostHogKey,
		"POSTHOG_HOST":         cfg.PostHogHost,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal runtime env: %w", err)
	}
	return append([]byte("window.__ENV__="), values...), nil
}

// Wrap serves web's paths and passes everything else to api.
func (h *Handler) Wrap(api http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.ownedByAPI(r) {
			api.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ownedByAPI reports whether the request belongs to the API: its /api and
// /.well-known prefixes (kamal-proxy no longer splits them off), the
// internal-network probes, and the api's custom domains.
func (h *Handler) ownedByAPI(r *http.Request) bool {
	p := r.URL.Path
	if p == "/api" || strings.HasPrefix(p, "/api/") ||
		strings.HasPrefix(p, "/.well-known/") || p == "/health" || p == "/metrics" {
		return true
	}
	host := r.Host
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i]
	}
	return h.apiHosts[host]
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	switch {
	case r.URL.Path == "/logs":
		h.serveLogs(w, r)
	case r.URL.Path == "/web-vitals":
		h.serveVitals(w, r)
	case r.Method != http.MethodGet && r.Method != http.MethodHead:
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	case r.URL.Path == "/env.js":
		h.serveBytes(w, r, h.envJS, "application/javascript; charset=utf-8", noStore)
	case r.URL.Path == "/sw.js":
		h.serveBytes(w, r, h.sw, "application/javascript; charset=utf-8", noStore)
	case r.URL.Path == "/release":
		h.serveRelease(w, r)
	default:
		h.serveStatic(w, r)
	}
}

func setSecurityHeaders(w http.ResponseWriter) {
	hdr := w.Header()
	hdr.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	hdr.Set("X-Frame-Options", "DENY")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

func (h *Handler) serveBytes(
	w http.ResponseWriter, r *http.Request, body []byte, contentType, cache string,
) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cache)
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	//nolint:gosec // body is server-side config or a file from the build
	_, _ = w.Write(body)
}

func (h *Handler) serveRelease(w http.ResponseWriter, r *http.Request) {
	body, _ := json.Marshal(map[string]string{"release": h.cfg.Release})
	h.serveBytes(w, r, body, "application/json", noStore)
}

// missingAssetExts are file types that 404 when absent, instead of falling
// back to the HTML shell (which would parse as a broken script or image).
//
//nolint:gochecknoglobals //read-only lookup
var missingAssetExts = map[string]bool{
	extJS: true, extMJS: true, ".css": true, ".map": true, ".png": true,
	".jpg": true, ".jpeg": true, ".svg": true, ".ico": true, ".webp": true,
	".woff": true, ".woff2": true, ".json": true, extManifest: true,
	".txt": true, ".zip": true, ".sig": true, ".wasm": true, ".xml": true,
}

func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean("/" + r.URL.Path)
	name := strings.TrimPrefix(clean, "/")

	if name != "" && name != indexFile && name != swFile {
		if info, err := h.root.Stat(name); err == nil && !info.IsDir() {
			h.serveFile(w, r, name, info.ModTime())
			return
		}
		if strings.HasPrefix(clean, "/assets/") || missingAssetExts[path.Ext(clean)] {
			http.NotFound(w, r)
			return
		}
	}

	h.serveIndex(w, r, clean)
}

func (h *Handler) serveFile(
	w http.ResponseWriter, r *http.Request, name string, modTime time.Time,
) {
	cache := revalidateCache
	if strings.HasPrefix(name, "assets/") {
		cache = immutableCache
	}
	w.Header().Set("Cache-Control", cache)

	if h.gzip.serve(w, r, h.root, name, modTime) {
		return
	}
	f, err := h.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, name, modTime, f)
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request, urlPath string) {
	body := h.indexFor(urlPath)
	w.Header().Set("Content-Security-Policy", h.csp)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", revalidateCache)
	if r.Method == http.MethodHead {
		return
	}
	//nolint:gosec // the shell comes from the build; only a fixed title is injected
	_, _ = w.Write(body)
}

var errNoLogsHandler = errors.New("no logs handler configured")

func (h *Handler) serveLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfg.Logs == nil {
		h.logger.WarnContext(r.Context(), "dropping client logs", "error", errNoLogsHandler)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !h.admit(w, r, h.logs, maxLogsBodyBytes) {
		return
	}
	h.cfg.Logs.ServeHTTP(w, r)
}
