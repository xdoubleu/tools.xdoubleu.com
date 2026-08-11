package books

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/services"
)

// coverCtxTimeout bounds the whole cover read path — R2 HeadObject, the DB
// lookup, the self-heal fetch+upload, and PresignGet — below the server's
// global 10s httpWriteTimeout (cmd/api/main.go). Without it, a slow or
// hanging R2/DB call can run past that write deadline: the write then
// silently fails (no error, no log line — see the deployLogsCtxTimeout
// comment in cmd/api/routes.go for the same failure mode), leaving the
// browser's <img> request hanging with neither a cover nor a clean error to
// trigger the placeholder. coverFetchTimeout (book_covers.go) only bounds
// the external cover-source fetch half of this path; this bounds the rest.
const coverCtxTimeout = 8 * time.Second

// coverRoutes mounts the public book-cover proxy endpoint.
// No auth is required — covers are public data and the response is suitable
// for CDN caching.
func (app *Books) coverRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/cover/{bookId}",
		app.coverHandler,
	)
}

// coverHandler handles GET /{prefix}/api/cover/{bookId}.
// Covers are fetched into R2 eagerly at write time (add/resync/merge — see
// BookService.cacheCoverFromURL), so this only ever reads R2: a hit issues a
// 302 redirect to a presigned URL, a miss returns 404.
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
