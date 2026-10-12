package learningpaths

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *LearningPaths) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connect.Server) {
			learningpathsv1connect.RegisterLearningPathsServiceHandler(
				s,
				&learningPathsConnectHandler{app: a},
			)
			learningpathsv1connect.RegisterTodoistServiceHandler(
				s,
				&todoistConnectHandler{app: a},
			)
		},
	)

	// The Todoist OAuth callback is a plain HTTP redirect, not ConnectRPC. It
	// needs only Auth.Access: the CSRF state, minted by the AppAccess-gated
	// ConnectTodoist, carries the identity.
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/oauth/todoist/callback", prefix),
		a.services.Auth.Access(a.todoistOAuthCallbackRoute()),
	)
}
