package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/internal/protomap"
	"tools.xdoubleu.com/apps/movies/internal/services"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "movies"

//nolint:lll // struct tags can't wrap
type mcpSearchArgs struct {
	Query  string `json:"query"            jsonschema:"title to search for on TMDB"`
	Shared bool   `json:"shared,omitempty" jsonschema:"read the family's shared list instead of the solo one"`
}

//nolint:lll // struct tags can't wrap
type mcpStatsArgs struct {
	Shared bool `json:"shared,omitempty" jsonschema:"read the family's shared list instead of the solo one"`
}

//nolint:lll // struct tags can't wrap
type mcpListBacklogArgs struct {
	Status       string `json:"status,omitempty"         jsonschema:"want, watching, watched or dropped; empty for all"`
	MediaType    string `json:"media_type,omitempty"     jsonschema:"movie or series; empty for both"`
	Sort         string `json:"sort,omitempty"           jsonschema:"added (default), title, release or rating"`
	NewSeason    bool   `json:"new_season,omitempty"     jsonschema:"only watched series with an aired season not yet ticked"`
	OnMyServices bool   `json:"on_my_services,omitempty" jsonschema:"only titles streaming (flatrate, free or ads) on the user's services in Belgium"`
	Limit        int32  `json:"limit,omitempty"          jsonschema:"max entries, default 50"`
	Offset       int32  `json:"offset,omitempty"         jsonschema:"pagination offset"`
	Shared       bool   `json:"shared,omitempty"         jsonschema:"read the family's shared list instead of the solo one"`
}

//nolint:lll // struct tags can't wrap
type mcpGetTitleArgs struct {
	ID     string `json:"id"               jsonschema:"backlog entry id"`
	Shared bool   `json:"shared,omitempty" jsonschema:"read the family's shared list instead of the solo one"`
}

type tools struct {
	movies *services.MovieService
}

// Register exposes the app's read and write tools, scoped to the caller.
func Register(srv *mcp.Server, svc *services.MovieService) {
	h := &tools{movies: svc}

	mcptools.AddReadTool(srv, mcpAppName, "movies_search_titles",
		"Search TMDB for movies and series. Results already in the user's "+
			"backlog carry their status.", h.mcpSearchTitles)
	mcptools.AddReadTool(srv, mcpAppName, "movies_list_backlog",
		"The user's movies & series backlog (or, with shared, the family's shared "+
			"one) with status, rating, watch dates and has_new_season and "+
			"on_my_services. Page with limit/offset while has_more is true.",
		h.mcpListBacklog)
	mcptools.AddReadTool(srv, mcpAppName, "movies_get_title",
		"A single backlog entry including the TMDB overview, seasons and where "+
			"to watch in Belgium (providers by offer type, data by JustWatch).", h.mcpGetTitle)
	mcptools.AddReadTool(srv, mcpAppName, "movies_get_settings",
		"The user's streaming services as TMDB provider IDs, used by the "+
			"on_my_services filter and flag.", h.mcpGetSettings)
	mcptools.AddReadTool(srv, mcpAppName, "movies_get_stats",
		"The user's (or, with shared, the family's shared) watching stats: dated "+
			"watches per month (last 12, "+
			"Europe/Brussels), top genres, rating distribution and totals.",
		h.mcpGetStats)

	// Writes: adr-0058.
	mcptools.AddWriteTool(srv, mcpAppName, "movies_add_title",
		"Add a movie or series to the backlog (or, with shared, the family's "+
			"shared one) by tmdb_id and media_type; movies_search_titles returns "+
			"both, and results already in the backlog carry their status. Adding "+
			"a title that is already there creates no duplicate: it returns the "+
			"existing entry and changes its status, rating or watch date only "+
			"when passed. New titles start as want.", h.mcpAddTitle)
	mcptools.AddWriteTool(srv, mcpAppName, "movies_update_title",
		"Change a backlog entry's status (want, watching, watched, dropped), "+
			"rating (1-5, 0 clears) or watch date, or tick/untick a series "+
			"season. The id comes from movies_list_backlog. Watched sets the "+
			"status and records a watch dated now unless watched_at is given.",
		h.mcpUpdateTitle)
	mcptools.AddWriteTool(srv, mcpAppName, "movies_remove_title",
		"Remove an entry from the backlog, including its rating and watch "+
			"dates. The id comes from movies_list_backlog.", h.mcpRemoveTitle)
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

// owner is the key of the caller's solo or, when shared, family list.
func (h *tools) owner(ctx context.Context, shared bool) (string, error) {
	uid, err := userID(ctx)
	if err != nil || !shared {
		return uid, err
	}
	owner, err := h.movies.Owner(ctx, uid, shared)
	return owner, mapError(err)
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid id",
		)
	}
	return id, nil
}

// target resolves the list owner and entry id of an entry tool.
func (h *tools) target(
	ctx context.Context,
	shared bool,
	rawID string,
) (string, uuid.UUID, error) {
	uid, err := h.owner(ctx, shared)
	if err != nil {
		return "", uuid.Nil, err
	}
	id, err := parseID(rawID)
	return uid, id, err
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrNotConfigured, connect.CodeFailedPrecondition),
	)
}

func (h *tools) mcpGetStats(
	ctx context.Context, args mcpStatsArgs,
) (proto.Message, error) {
	uid, err := h.owner(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	stats, err := h.movies.Stats(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.Stats(stats), nil
}

func (h *tools) mcpGetSettings(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.movies.Settings(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetSettingsResponse{ProviderIds: ids}, nil
}

func (h *tools) mcpSearchTitles(
	ctx context.Context, args mcpSearchArgs,
) (proto.Message, error) {
	uid, err := h.owner(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	results, err := h.movies.Search(ctx, uid, args.Query)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SearchTitlesResponse{
		Results: protomap.SearchResults(results),
	}, nil
}

func (h *tools) mcpListBacklog(
	ctx context.Context, args mcpListBacklogArgs,
) (proto.Message, error) {
	uid, err := h.owner(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	entries, hasMore, err := h.movies.List(ctx, uid, models.ListFilter{
		Status:       args.Status,
		MediaType:    args.MediaType,
		Sort:         args.Sort,
		NewSeason:    args.NewSeason,
		OnMyServices: args.OnMyServices,
		Limit:        args.Limit,
		Offset:       args.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.ListBacklogResponse{
		Entries: protomap.Entries(entries),
		HasMore: hasMore,
	}, nil
}

func (h *tools) mcpGetTitle(
	ctx context.Context, args mcpGetTitleArgs,
) (proto.Message, error) {
	uid, id, err := h.target(ctx, args.Shared, args.ID)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.Get(ctx, uid, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetTitleResponse{
		Entry:     protomap.Entry(entry),
		Overview:  entry.Title.Overview,
		Seasons:   protomap.Seasons(entry.Seasons),
		Offers:    protomap.Offers(entry.Providers),
		WatchLink: entry.Title.WatchLink,
	}, nil
}
