package kobogateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// GatewayVersion is the protocol version reported by /status. Bump it
	// whenever the gateway's HTTP API or file handling changes so the web
	// UI can trigger a self-update; routine releases don't bump it.
	GatewayVersion = 2

	// DefaultPort is the fixed port the web UI probes for a running gateway.
	DefaultPort = 41132

	// DefaultVolumesRoot is where macOS mounts USB drives.
	DefaultVolumesRoot = "/Volumes"

	// DefaultWebOrigin is the production web origin, used both as an
	// allowed origin and as the update subcommand's download origin.
	DefaultWebOrigin = "https://tools.xdoubleu.com"

	// DevWebOrigin is the local Next.js dev server origin.
	DevWebOrigin = "http://localhost:3000"

	contentTypeJSON = "application/json"
)

// DefaultAllowedOrigins returns the origins allowed to drive the gateway
// when no extra --allow-origin flags are passed.
func DefaultAllowedOrigins() []string {
	return []string{DefaultWebOrigin, DevWebOrigin}
}

// UpdateRunner downloads the latest gateway binary over the running one.
type UpdateRunner interface {
	SelfUpdate(ctx context.Context, origin string) error
}

// Config configures a gateway Server.
type Config struct {
	Port           int
	AllowedOrigins []string
	VolumesRoot    string
	Release        string
}

// Server is the localhost HTTP bridge driven by the books page.
type Server struct {
	cfg     Config
	updater UpdateRunner
	restart chan struct{}
}

// NewServer builds a Server; updater handles POST /update.
func NewServer(cfg Config, updater UpdateRunner) *Server {
	return &Server{
		cfg:     cfg,
		updater: updater,
		restart: make(chan struct{}, 1),
	}
}

// Restart is signalled once after a successful self-update; the caller
// should drain in-flight requests and re-exec the (replaced) binary.
func (s *Server) Restart() <-chan struct{} {
	return s.restart
}

// Handler returns the gateway's HTTP handler, all routes wrapped in the
// origin/host security middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", s.statusHandler)
	mux.HandleFunc("POST /configure", s.configureHandler)
	mux.HandleFunc("POST /revert", s.revertHandler)
	mux.HandleFunc("POST /update", s.updateHandler)

	return s.secure(mux)
}

// secure enforces the browser-facing security model: loopback Host only
// (DNS-rebinding guard), a strict Origin allowlist, and CORS/preflight
// handling including Chrome Private Network Access.
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden host")

			return
		}

		origin, ok := s.matchOrigin(r.Header.Get("Origin"))
		if !ok {
			writeError(w, http.StatusForbidden, "origin not allowed")

			return
		}

		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Cache-Control", "no-store")

		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, POST")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
				h.Set("Access-Control-Allow-Private-Network", "true")
			}
			w.WriteHeader(http.StatusNoContent)

			return
		}

		if r.Method == http.MethodPost &&
			!strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeJSON) {
			writeError(
				w,
				http.StatusBadRequest,
				"Content-Type must be application/json",
			)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) hostAllowed(host string) bool {
	return host == fmt.Sprintf("127.0.0.1:%d", s.cfg.Port) ||
		host == fmt.Sprintf("localhost:%d", s.cfg.Port)
}

// matchOrigin looks the request origin up in the allowlist and returns the
// matched allowlist entry. Callers must use the returned value, never the
// request header, wherever the origin is acted on (CORS echo, update
// download URL) — that keeps request data out of outbound requests.
func (s *Server) matchOrigin(origin string) (string, bool) {
	for _, allowed := range s.cfg.AllowedOrigins {
		if origin == allowed {
			return allowed, true
		}
	}

	return "", false
}

func (s *Server) statusHandler(w http.ResponseWriter, _ *http.Request) {
	kobos, err := FindKobos(s.cfg.VolumesRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{
		Version: GatewayVersion,
		Release: s.cfg.Release,
		Kobos:   kobos,
	})
}

func (s *Server) configureHandler(w http.ResponseWriter, r *http.Request) {
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")

		return
	}
	if !isAbsoluteHTTPURL(req.SyncURL) {
		writeError(w, http.StatusBadRequest, "syncUrl must be an absolute http(s) URL")

		return
	}

	kobo, status, err := s.resolveKobo(req.VolumePath)
	if err != nil {
		writeError(w, status, err.Error())

		return
	}

	original, err := s.setEndpoint(kobo, req.SyncURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, ConfigureResponse{
		Serial:           kobo.Serial,
		OriginalEndpoint: original,
	})
}

func (s *Server) revertHandler(w http.ResponseWriter, r *http.Request) {
	var req RevertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")

		return
	}
	if !isAbsoluteHTTPURL(req.TargetEndpoint) {
		writeError(
			w,
			http.StatusBadRequest,
			"targetEndpoint must be an absolute http(s) URL",
		)

		return
	}

	kobo, status, err := s.resolveKobo(req.VolumePath)
	if err != nil {
		writeError(w, status, err.Error())

		return
	}

	if _, err = s.setEndpoint(kobo, req.TargetEndpoint); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, RevertResponse{Serial: kobo.Serial})
}

// updateHandler downloads the latest binary from the requesting origin over
// the current executable, then signals a restart. The origin is re-matched
// against the allowlist so the download URL is built from the allowlist
// entry, not from request data (avoids request forgery).
func (s *Server) updateHandler(w http.ResponseWriter, r *http.Request) {
	origin, ok := s.matchOrigin(r.Header.Get("Origin"))
	if !ok {
		writeError(w, http.StatusForbidden, "origin not allowed")

		return
	}

	err := s.updater.SelfUpdate(r.Context(), origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())

		return
	}

	writeJSON(w, http.StatusOK, UpdateResponse{Updating: true})

	select {
	case s.restart <- struct{}{}:
	default:
	}
}

// resolveKobo picks the Kobo to act on: an explicit volumePath must match a
// detected Kobo; otherwise exactly one Kobo must be connected.
func (s *Server) resolveKobo(volumePath string) (Kobo, int, error) {
	var none Kobo

	kobos, err := FindKobos(s.cfg.VolumesRoot)
	if err != nil {
		return none, http.StatusInternalServerError, err
	}

	if volumePath != "" {
		for _, kobo := range kobos {
			if kobo.VolumePath == volumePath {
				return kobo, http.StatusOK, nil
			}
		}

		return none, http.StatusNotFound, errors.New(
			"no Kobo found at that volume path",
		)
	}

	switch len(kobos) {
	case 0:
		return none, http.StatusNotFound, errors.New("no Kobo volume found")
	case 1:
		return kobos[0], http.StatusOK, nil
	default:
		return none, http.StatusConflict, errors.New(
			"multiple Kobo volumes found, pass volumePath",
		)
	}
}

func (s *Server) setEndpoint(kobo Kobo, endpoint string) (string, error) {
	conf, err := readConfFile(kobo.VolumePath)
	if err != nil {
		return "", err
	}

	original := conf.SetAPIEndpoint(endpoint)
	if err = writeConfFile(kobo.VolumePath, conf); err != nil {
		return "", err
	}

	return original, nil
}

func isAbsoluteHTTPURL(raw string) bool {
	u, err := url.Parse(raw)

	return err == nil &&
		(u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
