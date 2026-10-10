package books

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// webPubRoutes serves the Readium manifest and single zip entries of the
// caller's original EPUB. The reader fetches them with its session cookie.
func (a *Books) webPubRoutes(prefix string, mux *http.ServeMux) {
	base := "/" + prefix + "/api/book/{bookId}/webpub/"
	mux.Handle(
		"GET "+base+"manifest.json",
		a.Services.Auth.AppAccess(prefix, a.webPubManifestHandler),
	)
	mux.Handle(
		"GET "+base+"res/{path...}",
		a.Services.Auth.AppAccess(prefix, a.webPubResourceHandler),
	)
}

func (a *Books) webPubRequest(
	w http.ResponseWriter,
	r *http.Request,
) (string, uuid.UUID, bool) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(), constants.UserContextKey,
	)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", uuid.Nil, false
	}
	bookID, err := uuid.Parse(r.PathValue("bookId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return "", uuid.Nil, false
	}
	return user.ID, bookID, true
}

func webPubError(w http.ResponseWriter, err error) {
	if errors.Is(err, database.ErrResourceNotFound) ||
		errors.Is(err, services.ErrWebPubResourceNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func (a *Books) webPubManifestHandler(w http.ResponseWriter, r *http.Request) {
	userID, bookID, ok := a.webPubRequest(w, r)
	if !ok {
		return
	}
	m, err := a.Services.WebPub.Manifest(r.Context(), userID, bookID)
	if err != nil {
		webPubError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/webpub+json")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_ = json.NewEncoder(w).Encode(m)
}

func (a *Books) webPubResourceHandler(w http.ResponseWriter, r *http.Request) {
	userID, bookID, ok := a.webPubRequest(w, r)
	if !ok {
		return
	}
	res, err := a.Services.WebPub.Resource(
		r.Context(), userID, bookID, r.PathValue("path"),
	)
	if err != nil {
		webPubError(w, err)
		return
	}
	defer func() { _ = res.Body.Close() }()
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(res.Size, 10))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, res.Body)
}
