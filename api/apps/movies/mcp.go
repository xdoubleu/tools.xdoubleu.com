package movies

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "movies"

type mcpSearchArgs struct {
	Query string `json:"query" jsonschema:"title to search for on TMDB"`
}

//nolint:lll // struct tags can't wrap
type mcpListBacklogArgs struct {
	Status    string `json:"status,omitempty"     jsonschema:"want, watching, watched or dropped; empty for all"`
	MediaType string `json:"media_type,omitempty" jsonschema:"movie or series; empty for both"`
	Sort      string `json:"sort,omitempty"       jsonschema:"added (default), title, release or rating"`
	NewSeason bool   `json:"new_season,omitempty" jsonschema:"only watched series with an aired season not yet ticked"`
	Limit     int32  `json:"limit,omitempty"      jsonschema:"max entries, default 50"`
	Offset    int32  `json:"offset,omitempty"     jsonschema:"pagination offset"`
}

type mcpGetTitleArgs struct {
	ID string `json:"id" jsonschema:"backlog entry id"`
}

// RegisterMCPTools exposes the app's read RPCs, scoped to the caller.
func (a *Movies) RegisterMCPTools(srv *mcp.Server) {
	h := &moviesConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "movies_search_titles",
		"Search TMDB for movies and series. Results already in the user's "+
			"backlog carry their status.", h.mcpSearchTitles)
	mcptools.AddReadTool(srv, mcpAppName, "movies_list_backlog",
		"The user's movies & series backlog with status, rating, watch dates and "+
			"has_new_season. Page with limit/offset while has_more is true.",
		h.mcpListBacklog)
	mcptools.AddReadTool(srv, mcpAppName, "movies_get_title",
		"A single backlog entry including the TMDB overview.", h.mcpGetTitle)
	mcptools.AddReadTool(srv, mcpAppName, "movies_get_stats",
		"The user's watching stats: dated watches per month (last 12, "+
			"Europe/Brussels), top genres, rating distribution and totals.",
		h.mcpGetStats)
}

func (h *moviesConnectHandler) mcpGetStats(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetStats(ctx, connect.NewRequest(
		&moviesv1.GetStatsRequest{},
	)))
}

func (h *moviesConnectHandler) mcpSearchTitles(
	ctx context.Context, args mcpSearchArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchTitles(ctx, connect.NewRequest(
		&moviesv1.SearchTitlesRequest{Query: args.Query},
	)))
}

func (h *moviesConnectHandler) mcpListBacklog(
	ctx context.Context, args mcpListBacklogArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListBacklog(ctx, connect.NewRequest(
		&moviesv1.ListBacklogRequest{
			Status:    args.Status,
			MediaType: args.MediaType,
			Sort:      args.Sort,
			NewSeason: args.NewSeason,
			Limit:     args.Limit,
			Offset:    args.Offset,
		},
	)))
}

func (h *moviesConnectHandler) mcpGetTitle(
	ctx context.Context, args mcpGetTitleArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetTitle(ctx, connect.NewRequest(
		&moviesv1.GetTitleRequest{Id: args.ID},
	)))
}
