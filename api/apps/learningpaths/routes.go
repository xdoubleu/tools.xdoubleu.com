package learningpaths

import (
	"fmt"
	"net/http"

	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *LearningPaths) Routes(prefix string, mux *http.ServeMux) {
	learningPathsPath, learningPathsHandler := learningpathsv1connect.
		NewLearningPathsServiceHandler(
			&learningPathsConnectHandler{app: a},
			iapp.ScrubInternalErrors(a.Logger),
		)
	mux.Handle(
		fmt.Sprintf("POST %s", learningPathsPath),
		a.services.Auth.AppAccess(prefix, learningPathsHandler.ServeHTTP),
	)

	todoistPath, todoistHandler := learningpathsv1connect.
		NewTodoistServiceHandler(
			&todoistConnectHandler{app: a},
			iapp.ScrubInternalErrors(a.Logger),
		)
	mux.Handle(
		fmt.Sprintf("POST %s", todoistPath),
		a.services.Auth.AppAccess(prefix, todoistHandler.ServeHTTP),
	)

	// The Todoist OAuth callback leg is a plain HTTP redirect route, not
	// ConnectRPC — Todoist's own browser redirect invokes it directly with
	// ?code=&state=, mirroring how api/cmd/api/oauth_admin.go's admin OAuth
	// callback can't fit Connect's POST-JSON/protobuf contract either. Gated
	// by Auth.Access (any logged-in user, not AppAccess) since the CSRF
	// state itself — minted only from within ConnectTodoist, which *is*
	// AppAccess-gated — carries the real identity the token gets stored
	// under; requiring app access again here would only reject an
	// already-authorized redirect the provider is delivering back to us.
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/oauth/todoist/callback", prefix),
		a.services.Auth.Access(a.todoistOAuthCallbackRoute()),
	)
}
