package books

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/internal/services/kobo"
	"tools.xdoubleu.com/internal/database"
)

// koboRoutes mounts the Kobo sync protocol under /{prefix}/kobo/{token}/. The
// token is a bearer secret in the device's api_endpoint, SHA-256 hashed before
// lookup; AppAccess is not used. Unhandled paths are proxied to the real Kobo
// store so firmware updates, purchases and auth keep working. The protocol
// itself lives in services/kobo; these handlers authenticate, parse and write.
func (app *Books) koboRoutes(prefix string, mux *http.ServeMux) {
	base := "/" + prefix + "/kobo/{token}"
	// The device fetches initialization with GET; serving POST too is harmless.
	mux.HandleFunc(
		"GET "+base+"/v1/initialization", app.koboLogged(app.koboInitHandler),
	)
	mux.HandleFunc(
		"POST "+base+"/v1/initialization", app.koboLogged(app.koboInitHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/sync", app.koboLogged(app.koboLibrarySyncHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/file",
		app.koboLogged(app.koboFileHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/{revisionId}/{width}/{height}/{greyscale}/image.jpg",
		app.koboLogged(app.koboCoverHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/{revisionId}/{width}/{height}/{quality}/{greyscale}/image.jpg",
		app.koboLogged(app.koboCoverHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/metadata",
		app.koboLogged(app.koboMetadataHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/state",
		app.koboLogged(app.koboGetStateHandler),
	)
	mux.HandleFunc(
		"PUT "+base+"/v1/library/{revisionId}/state",
		app.koboLogged(app.koboPutStateHandler),
	)
	mux.HandleFunc(
		"/"+prefix+"/kobo/{token}/", app.koboLogged(app.koboProxyHandler),
	)
}

// koboAuth validates HTTPS and the URL token. On false it has already written
// the error response.
func (app *Books) koboAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, _, ok := app.koboAuthDevice(w, r)
	return userID, ok
}

// koboAuthDevice is koboAuth that also returns the calling device's ID.
func (app *Books) koboAuthDevice(
	w http.ResponseWriter,
	r *http.Request,
) (string, string, bool) {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	if proto != "https" {
		http.Error(w, "https required", http.StatusForbidden)
		return "", "", false
	}

	raw := r.PathValue("token")
	if raw == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", "", false
	}

	userID, deviceID, err := app.services.Kobo.GetKoboAuthByTokenHash(
		r.Context(), kobo.HashToken(raw),
	)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return "", "", false
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return "", "", false
	}

	// Arm debug capture here: this runs before any handler touches the body.
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.deviceID = deviceID
		holder.enabled = app.services.KoboLog.IsEnabled(deviceID)
	}
	return userID, deviceID, true
}

func koboWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// koboCoverBase and koboLibraryBase are the device-facing URL prefixes for r.
func (app *Books) koboCoverBase(r *http.Request) string {
	return app.kobo.CoverBase(
		r.URL.Path, r.Header.Get("X-Forwarded-Host"), r.Host,
	)
}

func (app *Books) koboLibraryBase(r *http.Request) string {
	return app.kobo.LibraryBase(
		r.URL.Path, r.Header.Get("X-Forwarded-Host"), r.Host,
	)
}

// koboInitHandler handles POST /v1/initialization.
func (app *Books) koboInitHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}
	// The x-kobo-apitoken header proves the store handshake to the firmware;
	// without it the device treats init as incomplete and never advances to
	// /v1/library/sync. Set before koboWriteJSON writes the response.
	w.Header().Set("x-kobo-apitoken", "e30=")
	koboWriteJSON(w, app.kobo.Init(app.koboCoverBase(r)))
}

// koboLibrarySyncHandler handles GET /v1/library/sync, adding our kobo-sync
// books to the upstream store's entitlements.
func (app *Books) koboLibrarySyncHandler(w http.ResponseWriter, r *http.Request) {
	userID, deviceID, ok := app.koboAuthDevice(w, r)
	if !ok {
		return
	}

	res, err := app.kobo.Sync(r.Context(), kobo.SyncRequest{
		UserID:      userID,
		DeviceID:    deviceID,
		LibraryBase: app.koboLibraryBase(r),
		Header:      r.Header,
		RawQuery:    r.URL.RawQuery,
	})
	if res.UpstreamFetched {
		koboSetUpstreamHeaders(r, res.Upstream.Hdrs)
		koboSetUpstreamNote(r, w, res.Upstream.Note)
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	for name, vs := range res.Upstream.PagingHeaders() {
		w.Header()[name] = vs
	}
	koboWriteJSON(w, res.Entries)
}

// koboProxyHandler proxies paths we don't own to the upstream Kobo store,
// stripping the token segment.
func (app *Books) koboProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}

	token := r.PathValue("token")
	_, koboPath, _ := strings.Cut(r.URL.Path, "/kobo/"+token)

	resp, err := app.kobo.Proxy(
		r.Context(), r.Method, koboPath, r.URL.RawQuery, r.Header, r.Body,
	)
	if err != nil {
		if errors.Is(err, kobo.ErrBuildRequest) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		koboSetUpstreamNote(r, w, "upstream: "+err.Error())
		app.Logger.Warn("kobo upstream proxy failed", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	koboSetUpstreamHeaders(r, resp.Header)
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		koboSetUpstreamNote(r, w, kobo.UpstreamNote(resp.StatusCode, r.Header))
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// koboMetadataHandler handles GET /v1/library/{revisionId}/metadata, serving
// our kobo-sync books locally and proxying the rest upstream.
func (app *Books) koboMetadataHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	meta, err := app.kobo.Metadata(r.Context(), userID, bookID, app.koboLibraryBase(r))
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			app.koboProxyHandler(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	koboWriteJSON(w, meta)
}

// koboFileHandler handles GET /v1/library/{revisionId}/file with a 302 to a
// presigned R2 URL to the KEPUB.
func (app *Books) koboFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	result, err := app.services.Books.GetBookFile(
		r.Context(), userID, bookID, models.FileFormatKEPUB,
	)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, result.URL, http.StatusFound)
}

// koboUpstreamCoverURL is the Kobo CDN URL for the request's image.
func koboUpstreamCoverURL(r *http.Request) string {
	return kobo.UpstreamCoverURL(kobo.CoverParams{
		ImageID:   r.PathValue("revisionId"),
		Width:     r.PathValue("width"),
		Height:    r.PathValue("height"),
		Quality:   r.PathValue("quality"),
		Greyscale: r.PathValue("greyscale"),
	})
}

// koboCoverHandler serves a kobo-sync book's cached cover as a 302 to a
// presigned R2 URL (mirroring coverHandler); any other image id redirects to
// the Kobo CDN.
// The cached image is our single highest-resolution cover, so a full-screen
// sleep request gets the best available image regardless of the requested
// dimensions; quality stays bounded by the source cover (never upscaled).
func (app *Books) koboCoverHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	// The image templates route every cover here, so store-bought books' ids
	// (anything not one of our kobo-sync books) go to the Kobo CDN.
	bookID, err := kobo.CoverBookID(r.PathValue("revisionId"))
	if err != nil {
		http.Redirect(w, r, koboUpstreamCoverURL(r), http.StatusFound)
		return
	}

	owned, err := app.kobo.HasSyncBook(r.Context(), userID, bookID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !owned {
		http.Redirect(w, r, koboUpstreamCoverURL(r), http.StatusFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), coverCtxTimeout)
	defer cancel()

	result, err := app.services.Books.GetUserBookCover(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, services.ErrCoverNotFound) {
			http.Error(w, "cover not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Let the device cache the presigned redirect across syncs (the URL is
	// valid for 24 h, well inside the cache window).
	w.Header().
		Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")

	http.Redirect(w, r, result.URL, http.StatusFound)
}
