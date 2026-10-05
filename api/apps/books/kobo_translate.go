package books

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// koboWithSpans gives each KEPUB book's state that has a neutral EPUB position
// but no Kobo bookmark the KoboSpan location it translates to, replacing the
// entry in states. pending holds books whose span map is still building.
func (app *Books) koboWithSpans(
	ctx context.Context,
	userID string,
	kepubBooks []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
) map[uuid.UUID]bool {
	positions := make(map[uuid.UUID]models.ReadingPosition)
	for _, id := range kepubBooks {
		st := states[id]
		if st != nil && st.KoboLocation == nil && st.Position != nil &&
			st.Position.Href != "" {
			positions[id] = *st.Position
		}
	}
	if len(positions) == 0 {
		return nil
	}

	locs, pending := app.Services.Positions.KoboLocations(ctx, userID, positions)
	for id, loc := range locs {
		withLoc := *states[id]
		withLoc.KoboLocation = loc
		states[id] = &withLoc
	}
	return pending
}
