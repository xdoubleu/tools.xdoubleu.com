package goaltracker

import (
	"fmt"
	"net/http"
)

func (app *GoalTracker) apiRoutes(prefix string, mux *http.ServeMux) {
	apiPrefix := fmt.Sprintf("/%s/api", prefix)
	app.goalsRoutes(apiPrefix, mux)
	app.progressRoutes(apiPrefix, mux)
}

func (app *GoalTracker) Routes(prefix string, mux *http.ServeMux) {
	app.templateRoutes(prefix, mux)
	app.apiRoutes(prefix, mux)
}
