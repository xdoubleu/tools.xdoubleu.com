package movies

import (
	"context"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/pagination"
)

// ListConsumption returns userID's watching, watched and dropped titles as
// consumption events, for the taste app; userID may be a family's shared
// subject. Series and movies are separate
// domains; a series is dated and judged by its season watches.
func (a *Movies) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	seasons, err := a.services.Movies.SeasonWatchSummaries(ctx, userID)
	if err != nil {
		return nil, err
	}

	var events []consumption.Event
	for offset := int32(0); ; offset += pagination.MaxLimit {
		entries, hasMore, listErr := a.services.Movies.List(
			ctx, userID, models.ListFilter{
				Status: "", MediaType: "", Sort: models.SortAdded,
				NewSeason: false, OnMyServices: false,
				Limit: pagination.MaxLimit, Offset: offset,
			},
		)
		if listErr != nil {
			return nil, listErr
		}
		for _, e := range entries {
			if event, ok := entryEvent(e, seasons[e.ID]); ok {
				events = append(events, event)
			}
		}
		if !hasMore {
			return events, nil
		}
	}
}

// entryEvent maps an entry; seasons is the zero summary for movies. An entry
// with no dated watch stays undated.
func entryEvent(
	e models.Entry,
	seasons models.SeasonWatchSummary,
) (consumption.Event, bool) {
	rewatched := len(e.WatchedAt) > 1
	at := consumption.Latest(e.WatchedAt)
	domain := consumption.DomainMovies
	if e.Title.MediaType == "series" {
		domain = consumption.DomainSeries
		rewatched = seasons.Rewatched
		at = consumption.Latest([]*time.Time{seasons.LatestAt})
	}
	signal, reason, ok := entrySignal(e.Status, e.Rating, rewatched)
	if !ok {
		return consumption.Event{}, false //nolint:exhaustruct // unused when !ok
	}
	return consumption.Event{
		Domain:  domain,
		ItemID:  e.Title.MediaType + ":" + strconv.FormatInt(e.Title.TMDBID, 10),
		Title:   e.Title.Title,
		Creator: "",
		Tags:    e.Title.Genres,
		At:      at,
		Signal:  signal,
		Reason:  reason,
	}, true
}

func entrySignal(
	status string,
	rating *int,
	rewatched bool,
) (consumption.Signal, string, bool) {
	if status == models.StatusWant {
		return 0, "", false
	}
	if rating != nil {
		return consumption.RatingSignal(*rating), consumption.RatingReason(*rating), true
	}
	switch status {
	case models.StatusDropped:
		return consumption.Negative, "dropped", true
	case models.StatusWatching:
		return consumption.Neutral, "watching", true
	}
	if rewatched {
		return consumption.StrongPositive, "rewatched", true
	}
	return consumption.Positive, "watched", true
}

// ListSharedSubjects returns the shared-list subject of every family with
// a title, for the taste app's shared profiles.
func (a *Movies) ListSharedSubjects(ctx context.Context) ([]string, error) {
	return a.services.Movies.SharedSubjects(ctx)
}
