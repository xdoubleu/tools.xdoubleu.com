package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/movies/internal/protomap"
	"tools.xdoubleu.com/apps/movies/internal/services"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

//nolint:lll // struct tags can't wrap
type mcpAddTitleArgs struct {
	TMDBID    int64  `json:"tmdb_id"              jsonschema:"TMDB id, from movies_search_titles"`
	MediaType string `json:"media_type"           jsonschema:"movie or series"`
	Status    string `json:"status,omitempty"     jsonschema:"want, watching, watched or dropped. New titles default to want; for one already in the backlog the status only changes when passed"`
	Rating    *int   `json:"rating,omitempty"     jsonschema:"1-5 stars, 0 clears the rating"`
	WatchedAt string `json:"watched_at,omitempty" jsonschema:"YYYY-MM-DD date of the watch; movies only, with status watched"`
	Shared    bool   `json:"shared,omitempty"     jsonschema:"act on the family's shared list instead of the solo one"`
}

//nolint:lll // struct tags can't wrap
type mcpUpdateTitleArgs struct {
	ID            string `json:"id"                       jsonschema:"backlog entry id, from movies_list_backlog"`
	Status        string `json:"status,omitempty"         jsonschema:"want, watching, watched or dropped. Watched on a series ticks its aired seasons"`
	Rating        *int   `json:"rating,omitempty"         jsonschema:"1-5 stars, 0 clears the rating"`
	WatchedAt     string `json:"watched_at,omitempty"     jsonschema:"YYYY-MM-DD date of the watch: a movie's latest, or with season the ticked season's"`
	Season        *int   `json:"season,omitempty"         jsonschema:"series season number to tick or untick; needs season_watched"`
	SeasonWatched *bool  `json:"season_watched,omitempty" jsonschema:"true ticks the season, false unticks it"`
	Shared        bool   `json:"shared,omitempty"         jsonschema:"act on the family's shared list instead of the solo one"`
}

//nolint:lll // struct tags can't wrap
type mcpRemoveTitleArgs struct {
	ID     string `json:"id"               jsonschema:"backlog entry id, from movies_list_backlog"`
	Shared bool   `json:"shared,omitempty" jsonschema:"act on the family's shared list instead of the solo one"`
}

func invalidArgument(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, msg)
}

func (h *tools) mcpAddTitle(
	ctx context.Context, args mcpAddTitleArgs,
) (proto.Message, error) {
	change, err := services.NewTitleChange(args.Status, args.Rating, args.WatchedAt)
	if err != nil {
		return nil, mapError(err)
	}
	uid, err := h.owner(ctx, args.Shared)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.Upsert(ctx, uid, args.MediaType, args.TMDBID, change)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.AddTitleResponse{Entry: protomap.Entry(entry)}, nil
}

func (h *tools) mcpUpdateTitle(
	ctx context.Context, args mcpUpdateTitleArgs,
) (proto.Message, error) {
	change, err := services.NewTitleChange(args.Status, args.Rating, args.WatchedAt)
	if err != nil {
		return nil, mapError(err)
	}
	if (args.Season == nil) != (args.SeasonWatched == nil) {
		return nil, invalidArgument("season and season_watched go together")
	}
	uid, id, err := h.target(ctx, args.Shared, args.ID)
	if err != nil {
		return nil, err
	}
	var tick *services.SeasonTick
	if args.Season != nil {
		tick = &services.SeasonTick{Number: *args.Season, Watched: *args.SeasonWatched}
	}
	entry, err := h.movies.UpdateTitle(ctx, uid, id, change, tick)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetStatusResponse{Entry: protomap.Entry(entry)}, nil
}

func (h *tools) mcpRemoveTitle(
	ctx context.Context, args mcpRemoveTitleArgs,
) (proto.Message, error) {
	uid, id, err := h.target(ctx, args.Shared, args.ID)
	if err != nil {
		return nil, err
	}
	if err = h.movies.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.RemoveTitleResponse{}, nil
}
