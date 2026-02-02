package goaltracker

import (
	"fmt"
	"net/http"

	"github.com/xdoubleu/essentia/v2/pkg/parse"
)

func (app *GoalTracker) progressRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET %s/progress", prefix),
		app.Services.WebSocket.Handler(),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/progress/{id}/refresh", prefix),
		app.Services.Auth.Access(app.refreshProgressHandler),
	)
}
func (app *GoalTracker) refreshProgressHandler(_ http.ResponseWriter, r *http.Request) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	_, lastRunTime := app.jobQueue.FetchState(id)
	app.Services.WebSocket.UpdateState(id, true, lastRunTime)

	app.jobQueue.ForceRun(id)
}
