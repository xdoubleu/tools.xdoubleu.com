package games

import (
	"net/http"

	"tools.xdoubleu.com/apps/games/internal/jobs"
)

func (a *Games) refreshSteamHandler(w http.ResponseWriter, _ *http.Request) {
	a.Services.WebSocket.ForceRun(jobs.SteamJobID)
	w.WriteHeader(http.StatusNoContent)
}
