package backlog

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/backlog/internal/services"
)

// coverRoutes mounts the public book-cover proxy endpoint.
// No auth is required — covers are public data and the response is suitable
// for CDN caching.
func (app *Backlog) coverRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/cover/{bookId}",
		app.coverHandler,
	)
}

// coverHandler handles GET /{prefix}/api/cover/{bookId}.
// On a cache hit it issues a 302 redirect to a presigned R2 URL.
// On a cache miss it fetches the cover from Open Library, caches it in R2,
// then redirects. Returns 404 when no cover exists.
func (app *Backlog) coverHandler(w http.ResponseWriter, r *http.Request) {
	bookID, err := uuid.Parse(r.PathValue("bookId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	result, err := app.Services.Books.GetBookCover(r.Context(), bookID)
	if err != nil {
		if errors.Is(err, services.ErrCoverNotFound) {
			http.Error(w, "cover not found", http.StatusNotFound)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Allow browsers and CDNs to cache the redirect for 1 hour. The presigned
	// URL itself is valid for 24 h, so the cache window is well inside the TTL.
	w.Header().
		Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
	//nolint:gosec // URL comes from internal objectstore presign, not user input
	http.Redirect(w, r, result.URL, http.StatusFound)
}
