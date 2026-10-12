package feeds

import (
	"context"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// readInFullPct is the scroll depth from which a read counts as read in full.
const readInFullPct = 75

// ListConsumption returns userID's read, bookmarked and dismissed items as
// consumption events, for the taste app.
func (a *Feeds) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	items, err := a.Services.Feeds.ListEngaged(ctx, userID)
	if err != nil {
		return nil, err
	}

	events := make([]consumption.Event, 0, len(items))
	for _, item := range items {
		signal, reason := itemSignal(item)
		at := item.PublishedAt
		if item.ReadAt != nil {
			at = *item.ReadAt
		}
		events = append(events, consumption.Event{
			Domain:  consumption.DomainFeeds,
			ItemID:  item.ID.String(),
			Title:   item.Title,
			Creator: item.FeedTitle,
			Tags:    item.Categories,
			At:      at,
			Signal:  signal,
			Reason:  reason,
		})
	}
	return events, nil
}

func itemSignal(item models.EngagedItem) (consumption.Signal, string) {
	switch {
	case item.Bookmarked:
		return consumption.StrongPositive, "bookmarked"
	case item.ReadAt == nil:
		return consumption.Negative, "dismissed"
	case item.ReadProgressPct >= readInFullPct:
		return consumption.Positive, "read in full"
	default:
		return consumption.Neutral, "opened"
	}
}
