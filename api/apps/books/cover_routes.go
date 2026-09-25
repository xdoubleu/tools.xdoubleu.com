package books

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/services"
)

// coverCtxTimeout bounds the whole cover read path below the server's 10s
// write timeout; past it the write fails silently and the <img> hangs.
// coverFetchTimeout covers only the external fetch.
const coverCtxTimeout = 8 * time.Second

// coverRoutes mounts the public, CDN-cacheable cover proxy; no auth.
func (app *Books) coverRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/cover/{bookId}",
		app.coverHandler,
	)
}

// coverHandler handles GET /{prefix}/api/cover/{bookId}: 302 to a presigned
// R2 URL on a hit, 404 on a miss.
func (app *Books) coverHandler(w http.ResponseWriter, r *http.Request) {
	bookID, err := uuid.Parse(r.PathValue("bookId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), coverCtxTimeout)
	defer cancel()

	result, err := app.Services.Books.GetBookCover(ctx, bookID)
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

	http.Redirect(w, r, result.URL, http.StatusFound)
}
