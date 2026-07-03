package games

import "net/http"

func (a *Games) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
