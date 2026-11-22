package feeds

import (
	"net/http"

	connectv2 "connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/feeds/connect"
	feedshttp "tools.xdoubleu.com/apps/feeds/http"
	"tools.xdoubleu.com/apps/feeds/internal/resend"
	feedsmcp "tools.xdoubleu.com/apps/feeds/mcp"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	iapp "tools.xdoubleu.com/internal/app"
)

// SetResendAPIBaseURL overrides the Resend API base URL. Tests only.
func SetResendAPIBaseURL(u string) { resend.SetBaseURL(u) }

func (a *Feeds) Routes(prefix string, mux *http.ServeMux) {
	iapp.MountConnect(
		mux,
		a.Logger,
		func(next http.HandlerFunc) http.HandlerFunc {
			return a.Services.Auth.AppAccess(prefix, next)
		},
		func(s *connectv2.Server) {
			feedsv1connect.RegisterFeedServiceHandler(s, a.connectHandler())
		},
	)

	feedshttp.NewHandler(
		a.Services.Feeds,
		a.Logger,
		a.Config.EmailInboundSecret,
		a.Config.ResendAPIKey != "",
	).Routes(prefix, mux)
}

// RegisterMCPTools exposes the app's read-only RPCs as MCP tools, scoped to
// the caller.
func (a *Feeds) RegisterMCPTools(srv *mcp.Server) {
	feedsmcp.Register(srv, a.connectHandler())
}

func (a *Feeds) connectHandler() *connect.Handler {
	return connect.NewHandler(a.Services.Feeds, a.Logger)
}
