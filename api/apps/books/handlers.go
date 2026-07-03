package books

import "net/http"

func (app *Books) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
