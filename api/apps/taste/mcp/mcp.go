package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/protomap"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
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

type mcpAddFavouriteShowArgs struct {
	ITunesID int64 `json:"itunes_id" jsonschema:"the show's iTunes collection ID"`
}

//nolint:lll // struct tags can't wrap
type mcpRemoveFavouriteShowArgs struct {
	ID string `json:"id" jsonschema:"the favourite show's id, from taste_list_favourite_shows"`
}

type tools struct {
	svc *services.Services
}

// Register exposes the consumption log and its stats, scoped to the caller.
func Register(srv *mcp.Server, svc *services.Services) {
	h := &tools{svc: svc}

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
	// Show search stays out: it proxies a rate-limited external API.
	mcptools.AddReadTool(srv, mcpAppName, "taste_list_favourite_shows",
		"The user's favourite podcast shows, newest added first. They are the "+
			"only signal of the podcasts domain.", h.mcpListFavouriteShows)
	mcptools.AddWriteTool(srv, mcpAppName, "taste_add_favourite_show",
		"Favourite a podcast show by its iTunes collection ID.",
		h.mcpAddFavouriteShow)
	mcptools.AddWriteTool(srv, mcpAppName, "taste_remove_favourite_show",
		"Remove a podcast show from the user's favourites.",
		h.mcpRemoveFavouriteShow)
}

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrNotConfigured, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrNoProfile, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrUpstream, connect.CodeUnavailable),
		connecttools.Code(services.ErrPickNotFound, connect.CodeNotFound),
	)
}

func (h *tools) scopeOf(ctx context.Context, shared bool) (services.Scope, error) {
	uid, err := userID(ctx)
	if err != nil {
		return services.Scope{}, err
	}
	sc, err := h.svc.Scope.Resolve(ctx, uid, shared)
	if err != nil {
		return services.Scope{}, mapError(err)
	}
	return sc, nil
}

func (h *tools) mcpListFavouriteShows(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	shows, err := h.svc.Shows.List(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.FavouriteShow, len(shows))
	for i := range shows {
		out[i] = protomap.FavouriteShow(&shows[i])
	}
	return &tastev1.ListFavouriteShowsResponse{Favourites: out}, nil
}

func (h *tools) mcpAddFavouriteShow(
	ctx context.Context, args mcpAddFavouriteShowArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	show, err := h.svc.Shows.Add(ctx, uid, args.ITunesID)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.AddFavouriteShowResponse{
		Favourite: protomap.FavouriteShow(show),
	}, nil
}

func (h *tools) mcpRemoveFavouriteShow(
	ctx context.Context, args mcpRemoveFavouriteShowArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(args.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid id")
	}
	if err = h.svc.Shows.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &tastev1.RemoveFavouriteShowResponse{}, nil
}

func (h *tools) mcpListRecommendations(
	ctx context.Context, args mcpListRecommendationsArgs,
) (proto.Message, error) {
	sc, err := h.scopeOf(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	recs, st, err := h.svc.Picks.Visible(
		ctx, sc, args.Domain, args.Track, args.SavedOnly,
	)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.Recommendation, 0, len(recs))
	for _, r := range recs {
		out = append(out, protomap.Recommendation(r))
	}
	emailNewPicks, err := h.svc.Notify.EmailNewPicks(ctx, sc.User)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.ListRecommendationsResponse{
		Recommendations: out,
		GeneratedAt:     protomap.TimePtr(st.GeneratedAt),
		LastError:       st.Error,
		EmailNewPicks:   emailNewPicks,
		Pending:         st.Pending,
		NextRefreshAt:   protomap.NextRefreshAt(st.RequestedAt),
	}, nil
}

func (h *tools) mcpGetProfile(
	ctx context.Context, args mcpSharedArgs,
) (proto.Message, error) {
	sc, err := h.scopeOf(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	out := &tastev1.GetProfileResponse{Configured: h.svc.Profiles.Configured()}
	if !out.Configured {
		return out, nil
	}
	p, err := h.svc.Profiles.Get(ctx, sc.Subject)
	if err != nil {
		return nil, mapError(err)
	}
	if p != nil {
		protomap.FillProfile(out, p)
	}
	return out, nil
}

func (h *tools) mcpGetStats(
	ctx context.Context, args mcpSharedArgs,
) (proto.Message, error) {
	sc, err := h.scopeOf(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	stats, rates, err := h.svc.Reads.Stats(ctx, sc)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.Stats(stats, rates), nil
}

func (h *tools) mcpListEvents(
	ctx context.Context, args mcpListEventsArgs,
) (proto.Message, error) {
	sc, err := h.scopeOf(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	page, hasMore, err := h.svc.Reads.Events(
		ctx, sc, consumption.Domain(args.Domain), args.Limit, args.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.Events(page, hasMore), nil
}
