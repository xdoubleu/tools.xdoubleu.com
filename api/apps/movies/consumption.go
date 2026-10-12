package movies

import (
	"context"
	"strconv"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/pagination"
)

// ListConsumption returns userID's watching, watched and dropped titles as
// consumption events, for the taste app. Series and movies are separate
// domains.
func (a *Movies) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	var events []consumption.Event
	for offset := int32(0); ; offset += pagination.MaxLimit {
		entries, hasMore, err := a.services.Movies.List(ctx, userID, models.ListFilter{
			Status: "", MediaType: "", Sort: models.SortAdded,
			NewSeason: false, OnMyServices: false,
			Limit: pagination.MaxLimit, Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if event, ok := entryEvent(e); ok {
				events = append(events, event)
			}
		}
		if !hasMore {
			return events, nil
		}
	}
}

func entryEvent(e models.Entry) (consumption.Event, bool) {
	signal, reason, ok := entrySignal(e)
	if !ok {
		return consumption.Event{}, false //nolint:exhaustruct // unused when !ok
	}
	domain := consumption.DomainMovies
	if e.Title.MediaType == "series" {
		domain = consumption.DomainSeries
	}
	at := consumption.Latest(e.WatchedAt)
	if at.IsZero() {
		at = e.UpdatedAt
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

func entrySignal(e models.Entry) (consumption.Signal, string, bool) {
	if e.Status == models.StatusWant {
		return 0, "", false
	}
	if e.Rating != nil {
		return consumption.RatingSignal(*e.Rating), consumption.RatingReason(*e.Rating), true
	}
	switch e.Status {
	case models.StatusDropped:
		return consumption.Negative, "dropped", true
	case models.StatusWatching:
		return consumption.Neutral, "watching", true
	}
	if len(e.WatchedAt) > 1 {
		return consumption.StrongPositive, "rewatched", true
	}
	return consumption.Positive, "watched", true
}
