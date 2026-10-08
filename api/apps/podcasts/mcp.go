package podcasts

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "podcasts"

//nolint:lll // struct tags can't wrap
type mcpListEpisodesArgs struct {
	ShowID string `json:"show_id,omitempty" jsonschema:"favourite show id from podcasts_list_favourites; empty for every favourite"`
	Limit  int32  `json:"limit,omitempty"   jsonschema:"max episodes, default 50"`
	Offset int32  `json:"offset,omitempty"  jsonschema:"pagination offset"`
}

// RegisterMCPTools exposes the app's read RPCs, scoped to the caller. Search
// and the write RPCs stay out: search proxies a rate-limited external API.
func (a *Podcasts) RegisterMCPTools(srv *mcp.Server) {
	h := &podcastsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "podcasts_list_favourites",
		"The user's favourite podcast shows, newest added first, with the last "+
			"feed fetch error if any.", h.mcpListFavourites)
	mcptools.AddReadTool(srv, mcpAppName, "podcasts_list_episodes",
		"Episodes of the user's favourite shows, newest first, with show, date, "+
			"duration, summary and links. Page with limit/offset while has_more is true.",
		h.mcpListEpisodes)
}

func (h *podcastsConnectHandler) mcpListFavourites(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFavourites(ctx, connect.NewRequest(
		&podcastsv1.ListFavouritesRequest{},
	)))
}

func (h *podcastsConnectHandler) mcpListEpisodes(
	ctx context.Context, args mcpListEpisodesArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListEpisodes(ctx, connect.NewRequest(
		&podcastsv1.ListEpisodesRequest{
			ShowId: args.ShowID,
			Limit:  args.Limit,
			Offset: args.Offset,
		},
	)))
}
