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

	// The Todoist OAuth callback is a plain HTTP redirect, not ConnectRPC. It
	// needs only Auth.Access: the CSRF state, minted by the AppAccess-gated
	// ConnectTodoist, carries the identity.
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/oauth/todoist/callback", prefix),
		a.services.Auth.Access(a.todoistOAuthCallbackRoute()),
	)
}
