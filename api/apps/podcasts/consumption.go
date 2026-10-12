package podcasts

import (
	"context"

	"tools.xdoubleu.com/internal/consumption"
)

// ListConsumption returns userID's favourite shows as consumption events, for
// the taste app. Favourites are the only listening signal.
func (a *Podcasts) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	shows, err := a.services.Shows.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	events := make([]consumption.Event, 0, len(shows))
	for _, s := range shows {
		events = append(events, consumption.Event{
			Domain:  consumption.DomainPodcasts,
			ItemID:  s.ID.String(),
			Title:   s.Title,
			Creator: s.Author,
			Tags:    nil,
			At:      s.AddedAt,
			Signal:  consumption.StrongPositive,
			Reason:  "favourite",
		})
	}
	return events, nil
}
