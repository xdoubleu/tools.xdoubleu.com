package taste

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "taste"

//nolint:lll // struct tags can't wrap
type mcpListEventsArgs struct {
	Domain string `json:"domain,omitempty" jsonschema:"books, feeds, games, movies, series or podcasts; empty for every domain"`
	Limit  int32  `json:"limit,omitempty"  jsonschema:"max events, default 50"`
	Offset int32  `json:"offset,omitempty" jsonschema:"pagination offset"`
}

// RegisterMCPTools exposes the consumption log and its stats, scoped to the
// caller.
func (a *Taste) RegisterMCPTools(srv *mcp.Server) {
	h := &tasteConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "taste_get_stats",
		"The user's consumption across books, feeds, games, movies, series and "+
			"podcasts: per-domain totals with positive/negative counts, events per "+
			"month for the last 12 months (Europe/Brussels), and the top tags and "+
			"creators by positively rated items.", h.mcpGetStats)
	mcptools.AddReadTool(srv, mcpAppName, "taste_list_events",
		"The user's consumption log, newest first: each consumed item with its "+
			"domain, creator, tags, date and a signal from -2 (strong dislike) to 2 "+
			"(strong like) normalized within its domain, plus the reason. Page "+
			"with limit/offset while has_more is true.", h.mcpListEvents)
	mcptools.AddReadTool(srv, mcpAppName, "taste_get_profile",
		"The user's LLM-written taste profile: a summary plus what they know, "+
			"what they like, where their taste is drifting and what they bounce "+
			"off, with when and by which model it was written.", h.mcpGetProfile)
}

func (h *tasteConnectHandler) mcpGetProfile(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetProfile(ctx, connect.NewRequest(
		&tastev1.GetProfileRequest{},
	)))
}

func (h *tasteConnectHandler) mcpGetStats(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetStats(ctx, connect.NewRequest(
		&tastev1.GetStatsRequest{},
	)))
}

func (h *tasteConnectHandler) mcpListEvents(
	ctx context.Context, args mcpListEventsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListEvents(ctx, connect.NewRequest(
		&tastev1.ListEventsRequest{
			Domain: args.Domain,
			Limit:  args.Limit,
			Offset: args.Offset,
		},
	)))
}
