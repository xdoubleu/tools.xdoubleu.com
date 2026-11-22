package learningpaths

import (
	"fmt"
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/learningpaths/connect"
	lpmcp "tools.xdoubleu.com/apps/learningpaths/mcp"
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
		func(s *connectv2.Server) {
			learningpathsv1connect.RegisterLearningPathsServiceHandler(
				s,
				connect.NewHandler(a.services.LearningPaths),
			)
			learningpathsv1connect.RegisterTodoistServiceHandler(
				s,
				connect.NewTodoistHandler(a.services.Todoist),
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

// RegisterMCPTools exposes four read and three write tools on the combined
// apps MCP server.
func (a *LearningPaths) RegisterMCPTools(srv *mcp.Server) {
	lpmcp.Register(srv, a.services.LearningPaths, a.services.Todoist)
}
