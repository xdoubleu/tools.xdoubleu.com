package movies

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/models"
)

// WatchingTitle is a title being watched, for the Today page.
type WatchingTitle struct {
	ID           string
	Title        string
	HasNewSeason bool
}

// ListWatching returns up to limit of userID's titles being watched.
func (app *Movies) ListWatching(
	ctx context.Context,
	userID string,
	limit int32,
) ([]WatchingTitle, error) {
	entries, _, err := app.services.Movies.List(ctx, userID, models.ListFilter{
		Status:       models.StatusWatching,
		MediaType:    "",
		Sort:         "",
		NewSeason:    false,
		OnMyServices: false,
		Limit:        limit,
		Offset:       0,
	})
	if err != nil {
		return nil, err
	}

	out := make([]WatchingTitle, 0, len(entries))
	for _, e := range entries {
		out = append(out, WatchingTitle{
			ID:           e.ID.String(),
			Title:        e.Title.Title,
			HasNewSeason: e.HasNewSeason,
		})
	}
	return out, nil
}
