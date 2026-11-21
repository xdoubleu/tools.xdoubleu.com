package movies

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
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

// titleChange is the validated part of an add or update that edits an entry.
type titleChange struct {
	status    string
	rating    *int
	watchedAt *time.Time
}

func invalidArgument(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, msg)
}

// newTitleChange validates the optional status, rating and watched_at.
func newTitleChange(status string, rating *int, watchedAt string) (titleChange, error) {
	change := titleChange{status: status, rating: nil, watchedAt: nil}
	if status != "" && !models.IsStatus(status) {
		return change, invalidArgument(
			"status must be want, watching, watched or dropped",
		)
	}
	if rating != nil && (*rating < 0 || *rating > 5) {
		return change, invalidArgument("rating must be 0 to 5 (0 clears it)")
	}
	change.rating = rating
	if watchedAt != "" {
		day, err := time.Parse(time.DateOnly, watchedAt)
		if err != nil {
			return change, invalidArgument("watched_at must be YYYY-MM-DD")
		}
		change.watchedAt = &day
	}
	return change, nil
}

func (h *moviesConnectHandler) mcpAddTitle(
	ctx context.Context, args mcpAddTitleArgs,
) (proto.Message, error) {
	change, err := newTitleChange(args.Status, args.Rating, args.WatchedAt)
	if err != nil {
		return nil, err
	}
	uid, err := h.owner(ctx, args.Shared)
	if err != nil {
		return nil, err
	}

	entry, _, err := h.app.services.Movies.Ensure(
		ctx, uid, args.MediaType, args.TMDBID,
	)
	if err != nil {
		return nil, mapError(err)
	}
	entry, err = h.applyChange(ctx, uid, entry, change)
	if err != nil {
		return nil, err
	}
	return &moviesv1.AddTitleResponse{Entry: protoEntry(entry)}, nil
}

func (h *moviesConnectHandler) mcpUpdateTitle(
	ctx context.Context, args mcpUpdateTitleArgs,
) (proto.Message, error) {
	change, err := newTitleChange(args.Status, args.Rating, args.WatchedAt)
	if err != nil {
		return nil, err
	}
	if (args.Season == nil) != (args.SeasonWatched == nil) {
		return nil, invalidArgument("season and season_watched go together")
	}
	uid, id, err := h.target(ctx, args.Shared, args.ID)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.Get(ctx, uid, id)
	if err != nil {
		return nil, mapError(err)
	}
	if args.Season != nil {
		entry, err = h.tickSeason(ctx, uid, entry, *args.Season, *args.SeasonWatched,
			change.watchedAt)
		if err != nil {
			return nil, err
		}
		change.watchedAt = nil
	}
	entry, err = h.applyChange(ctx, uid, entry, change)
	if err != nil {
		return nil, err
	}
	return &moviesv1.SetStatusResponse{Entry: protoEntry(entry)}, nil
}

func (h *moviesConnectHandler) mcpRemoveTitle(
	ctx context.Context, args mcpRemoveTitleArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.RemoveTitle(ctx, &moviesv1.RemoveTitleRequest{
		Id: args.ID, Shared: args.Shared,
	}))
}

// applyChange applies the change to the entry and returns it as stored.
// A status equal to the entry's is a no-op.
func (h *moviesConnectHandler) applyChange(
	ctx context.Context,
	uid string,
	entry *models.Entry,
	change titleChange,
) (*models.Entry, error) {
	svc := h.app.services.Movies
	if change.watchedAt != nil && entry.Title.MediaType != tmdb.MediaTypeMovie {
		return nil, invalidArgument(
			"watched_at is for movies; for a series pass season and season_watched",
		)
	}
	if change.watchedAt != nil && change.status != models.StatusWatched &&
		(change.status != "" || entry.Status != models.StatusWatched) {
		return nil, invalidArgument("watched_at needs status watched")
	}

	var err error
	if change.status != "" && change.status != entry.Status {
		if entry, err = svc.SetStatus(
			ctx, uid, entry.ID, change.status, false,
		); err != nil {
			return nil, mapError(err)
		}
	}
	if change.rating != nil {
		rating := change.rating
		if *rating == 0 {
			rating = nil
		}
		if entry, err = svc.SetRating(ctx, uid, entry.ID, rating); err != nil {
			return nil, mapError(err)
		}
	}
	if change.watchedAt != nil {
		entry, err = h.dateLatestWatch(ctx, uid, entry, nil, change.watchedAt)
		if err != nil {
			return nil, err
		}
	}
	return entry, nil
}

// dateLatestWatch dates the entry's (or the season's) latest watch, adding a
// watch when there is none.
func (h *moviesConnectHandler) dateLatestWatch(
	ctx context.Context,
	uid string,
	entry *models.Entry,
	season *int,
	day *time.Time,
) (*models.Entry, error) {
	svc := h.app.services.Movies
	watches := entry.WatchedAt
	if season != nil {
		watches = nil
		for _, s := range entry.Seasons {
			if s.Number == *season {
				watches = s.WatchedAt
			}
		}
	}
	var (
		out *models.Entry
		err error
	)
	if len(watches) == 0 {
		out, err = svc.AddWatchDate(ctx, uid, entry.ID, season, day)
	} else {
		out, err = svc.EditWatchDate(ctx, uid, entry.ID, season, len(watches)-1, day)
	}
	return out, mapError(err)
}

// tickSeason ticks or unticks a series season, dating the new watch when day
// is set.
func (h *moviesConnectHandler) tickSeason(
	ctx context.Context,
	uid string,
	entry *models.Entry,
	season int,
	watched bool,
	day *time.Time,
) (*models.Entry, error) {
	if entry.Title.MediaType != tmdb.MediaTypeSeries {
		return nil, invalidArgument("season only applies to series")
	}
	if day != nil && !watched {
		return nil, invalidArgument("watched_at needs season_watched true")
	}
	out, err := h.app.services.Movies.SetSeasonWatched(
		ctx, uid, entry.ID, season, watched, false,
	)
	if err != nil {
		return nil, mapError(err)
	}
	if day == nil {
		return out, nil
	}
	return h.dateLatestWatch(ctx, uid, out, &season, day)
}
