package books

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"tools.xdoubleu.com/apps/books/internal/services/kobo"
)

// koboGetStateHandler handles GET /v1/library/{revisionId}/state; a Kobo store
// book's state is the store's.
func (app *Books) koboGetStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, isLibrary, err := app.kobo.LibraryBookID(
		r.Context(), userID, r.PathValue("revisionId"),
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isLibrary {
		app.koboProxyHandler(w, r)
		return
	}

	state, err := app.kobo.GetState(r.Context(), userID, bookID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	koboWriteJSON(w, state)
}

// koboPutStateHandler handles PUT /v1/library/{revisionId}/state. A Kobo store
// book's state goes to the store (koboStorePutState).
func (app *Books) koboPutStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, deviceID, ok := app.koboAuthDevice(w, r)
	if !ok {
		return
	}

	id := r.PathValue("revisionId")
	bookID, isLibrary, err := app.kobo.LibraryBookID(r.Context(), userID, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isLibrary {
		app.koboStorePutState(w, r, userID, id)
		return
	}

	ack, err := app.kobo.PutState(r.Context(), userID, deviceID, bookID, r.Body)
	switch {
	case errors.Is(err, kobo.ErrBadStateBody):
		http.Error(w, "bad request", http.StatusBadRequest)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		koboWriteJSON(w, ack)
	}
}

// koboStorePutState proxies a store book's state PUT upstream, then mirrors
// it onto the library in the background.
func (app *Books) koboStorePutState(
	w http.ResponseWriter,
	r *http.Request,
	userID, id string,
) {
	body, err := io.ReadAll(io.LimitReader(r.Body, kobo.MaxStateBody))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	app.koboProxyHandler(w, r)

	app.kobo.MirrorStoreStateAsync(r.Context(), userID, id, r.Header.Clone(), body)
}
