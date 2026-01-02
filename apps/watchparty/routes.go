package watchparty

import (
	"fmt"
	"net/http"
)

func (app *WatchParty) apiRoutes(prefix string, mux *http.ServeMux) {
	apiPrefix := fmt.Sprintf("/%s/api", prefix)

	app.wsRoutes(apiPrefix, mux)
}

func (app *WatchParty) Routes(prefix string, mux *http.ServeMux) {
	app.templateRoutes(prefix, mux)
	app.apiRoutes(prefix, mux)
}
