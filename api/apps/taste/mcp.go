package taste

import (
	"context"

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
	Shared bool   `json:"shared,omitempty" jsonschema:"read the family's shared movies & series taste instead of the user's own"`
}

//nolint:lll // struct tags can't wrap
type mcpSharedArgs struct {
	Shared bool `json:"shared,omitempty" jsonschema:"read the family's shared movies & series taste instead of the user's own"`
}

//nolint:lll // struct tags can't wrap
type mcpListRecommendationsArgs struct {
	Domain    string `json:"domain,omitempty"     jsonschema:"books, feeds, games, movies, series or podcasts; empty for every domain"`
	Track     string `json:"track,omitempty"      jsonschema:"more (more of what you like) or broaden (broaden your horizon); empty for both"`
	SavedOnly bool   `json:"saved_only,omitempty" jsonschema:"every saved pick from any run instead of the latest run's"`
	Shared    bool   `json:"shared,omitempty"     jsonschema:"read the family's shared movies & series taste instead of the user's own"`
}

// RegisterMCPTools exposes the consumption log and its stats, scoped to the
// caller.
func (a *Taste) RegisterMCPTools(srv *mcp.Server) {
	h := &tasteConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "taste_get_stats",
		"The user's consumption across books, feeds, games, movies, series and "+
			"podcasts: per-domain totals with positive/negative counts, events per "+
			"month for the last 12 months (Europe/Brussels), and the top tags and "+
			"creators by positively rated items, plus each recommendation domain "+
			"and track's hits and misses.", h.mcpGetStats)
	mcptools.AddReadTool(srv, mcpAppName, "taste_list_events",
		"The user's consumption log, newest first: each consumed item with its "+
			"domain, creator, tags, date and a signal from -2 (strong dislike) to 2 "+
			"(strong like) normalized within its domain, plus the reason. Page "+
			"with limit/offset while has_more is true.", h.mcpListEvents)
	mcptools.AddReadTool(srv, mcpAppName, "taste_get_profile",
		"The user's LLM-written taste profile: a summary plus what they know, "+
			"what they like, where their taste is drifting and what they bounce "+
			"off, with when and by which model it was written.", h.mcpGetProfile)
	mcptools.AddReadTool(srv, mcpAppName, "taste_list_recommendations",
		"The user's current recommendations, best first: per domain and track "+
			"(more of what they like, or broaden their horizon), each verified "+
			"against a catalog with a one-line why, the model's 0-100 score, "+
			"the web sources the generator cited, and the user's feedback, "+
			"saved and accepted state. saved_only lists every saved pick.",
		h.mcpListRecommendations)
}

func (h *tasteConnectHandler) mcpListRecommendations(
	ctx context.Context, args mcpListRecommendationsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListRecommendations(ctx, &tastev1.ListRecommendationsRequest{
		Domain: args.Domain, Track: args.Track, SavedOnly: args.SavedOnly,
		Shared: args.Shared,
	}))
}

func (h *tasteConnectHandler) mcpGetProfile(
	ctx context.Context, args mcpSharedArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetProfile(ctx, &tastev1.GetProfileRequest{
		Shared: args.Shared,
	}))
}

func (h *tasteConnectHandler) mcpGetStats(
	ctx context.Context, args mcpSharedArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetStats(ctx, &tastev1.GetStatsRequest{
		Shared: args.Shared,
	}))
}

func (h *tasteConnectHandler) mcpListEvents(
	ctx context.Context, args mcpListEventsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListEvents(ctx, &tastev1.ListEventsRequest{
		Domain: args.Domain,
		Limit:  args.Limit,
		Offset: args.Offset,
		Shared: args.Shared,
	}))
}
