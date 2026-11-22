package books

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/books/connect"
	booksmcp "tools.xdoubleu.com/apps/books/mcp"
	booksv1connect "tools.xdoubleu.com/gen/books/v1/booksv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Books) booksRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		a.services.Auth.Access(a.services.WebSocket.Handler()),
	)

	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			handler := a.connectHandler()
			booksv1connect.RegisterLibraryServiceHandler(s, handler)
			booksv1connect.RegisterBookFilesServiceHandler(s, handler)
			booksv1connect.RegisterKoboServiceHandler(s, handler)
			booksv1connect.RegisterCatalogServiceHandler(s, handler)
		},
	)
}

func (a *Books) Routes(prefix string, mux *http.ServeMux) {
	a.booksRoutes(prefix, mux)
	a.coverRoutes(prefix, mux)
	a.webPubRoutes(prefix, mux)
	a.koboRoutes(prefix, mux)
}

// RegisterMCPTools exposes the books app's read-only RPCs on the combined apps
// MCP server. Every tool returns the calling user's own library data.
func (a *Books) RegisterMCPTools(srv *mcp.Server) {
	booksmcp.Register(
		srv,
		a.connectHandler(),
		a.services.Catalog,
		a.clients.PublicAPIBaseURL,
	)
}

func (a *Books) connectHandler() *connect.Handler {
	return connect.NewHandler(a.services, a.Logger, a.clients.PublicAPIBaseURL)
}
