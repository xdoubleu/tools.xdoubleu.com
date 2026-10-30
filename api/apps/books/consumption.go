package books

import (
	"context"
	"slices"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// maxAuthors caps the creator credit of a multi-author book.
const maxAuthors = 2

// ListConsumption returns userID's finished, dropped and currently-reading
// books as consumption events, for the taste app, dated by their last finish,
// or a book being read for the first time by its last update. URL-ingested
// articles aren't books and are left out.
func (app *Books) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	library, err := app.services.Books.GetLibrary(ctx, userID)
	if err != nil {
		return nil, err
	}

	events := make([]consumption.Event, 0, len(library))
	for _, ub := range library {
		if ub.Book == nil || ub.Book.SourceURL != nil {
			continue
		}
		signal, reason, ok := bookSignal(ub)
		if !ok {
			continue
		}
		var at time.Time
		switch {
		case len(ub.FinishedAt) > 0:
			at = slices.MaxFunc(ub.FinishedAt, time.Time.Compare)
		case ub.Status == models.StatusReading:
			at = ub.UpdatedAt
		}
		events = append(events, consumption.Event{
			Domain:  consumption.DomainBooks,
			ItemID:  ub.BookID.String(),
			Title:   ub.Book.Title,
			Creator: strings.Join(ub.Book.Authors[:min(len(ub.Book.Authors), maxAuthors)], ", "),
			Tags:    ub.DisplayTags(),
			At:      at,
			Signal:  signal,
			Reason:  reason,
		})
	}
	return events, nil
}

// bookSignal rates a read, dropped or currently-reading book; other statuses
// aren't consumed.
func bookSignal(ub models.UserBook) (consumption.Signal, string, bool) {
	switch ub.Status {
	case models.StatusRead, models.StatusDropped, models.StatusReading:
	default:
		return 0, "", false
	}
	if ub.Rating != nil {
		stars := int(*ub.Rating)
		return consumption.RatingSignal(stars), consumption.RatingReason(stars), true
	}
	if ub.Status == models.StatusDropped {
		return consumption.Negative, "abandoned", true
	}
	if ub.HasTag(models.TagFavourite) {
		return consumption.StrongPositive, "favourite", true
	}
	if len(ub.FinishedAt) > 1 {
		return consumption.StrongPositive, "re-read", true
	}
	if ub.Status == models.StatusReading {
		return consumption.Neutral, "reading", true
	}
	return consumption.Positive, "finished", true
}
