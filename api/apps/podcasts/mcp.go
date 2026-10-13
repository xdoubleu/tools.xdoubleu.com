package podcasts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "podcasts"

// RegisterMCPTools exposes the app's read RPCs, scoped to the caller. Search
// and the write RPCs stay out: search proxies a rate-limited external API.
func (a *Podcasts) RegisterMCPTools(srv *mcp.Server) {
	h := &podcastsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "podcasts_list_favourites",
		"The user's favourite podcast shows, newest added first.", h.mcpListFavourites)
}

func (h *podcastsConnectHandler) mcpListFavourites(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFavourites(ctx, &podcastsv1.ListFavouritesRequest{}))
}
