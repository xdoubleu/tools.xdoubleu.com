package books

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
)

// koboPutLocation sets a PUT state's bookmark and its translated position. A
// finished book's Source is the first resource, so it's kept as Value only.
// It reports whether the position should be backfilled once translatable.
func (app *Books) koboPutLocation(
	ctx context.Context,
	state *models.BookReadingState,
	raw json.RawMessage,
	status string,
) bool {
	loc := parseKoboLocation(raw)
	if loc == nil {
		return false
	}
	state.Location = &loc.Value
	if loc.Source == "" || status == koboStatusFinished {
		return false
	}
	state.KoboLocation = loc
	var backfill bool
	state.Position, backfill = app.Services.Positions.KoboPosition(
		ctx, state.UserID, state.BookID, *loc,
	)
	return backfill
}

// koboSyncSpans translates the states deviceID lacks (it ignores the state in
// a repeated NewEntitlement) and those of regenerated KEPUBs, returning what
// it holds and the pending books.
func (app *Books) koboSyncSpans(
	ctx context.Context,
	userID, deviceID string,
	books []models.KoboSyncBook,
	states map[uuid.UUID]*models.BookReadingState,
) (map[uuid.UUID]time.Time, map[uuid.UUID]bool, error) {
	held, err := app.Services.Books.ListKoboDeviceHeld(ctx, deviceID)
	if err != nil {
		return nil, nil, err
	}
	var lacking []uuid.UUID
	for _, b := range books {
		if b.Format == models.FileFormatKEPUB && (koboIsReplace(b) ||
			services.KoboDeviceLacks(held, b.BookID, states[b.BookID])) {
			lacking = append(lacking, b.BookID)
		}
	}
	return held, app.koboWithSpans(ctx, userID, lacking, states), nil
}

// koboIsReplace reports whether b's file was regenerated since it was last
// sent, so it goes out as a ChangedEntitlement.
func koboIsReplace(b models.KoboSyncBook) bool {
	return b.LastSyncedConverterVersion != nil &&
		*b.LastSyncedConverterVersion != b.ConverterVersion
}

// koboWithSpans replaces each KEPUB book's state in states with one carrying
// the KoboSpan location to send (PositionService.KoboLocations). pending
// holds books whose span map is still building.
func (app *Books) koboWithSpans(
	ctx context.Context,
	userID string,
	kepubBooks []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
) map[uuid.UUID]bool {
	sub := make(map[uuid.UUID]*models.BookReadingState, len(kepubBooks))
	for _, id := range kepubBooks {
		if st := states[id]; st != nil {
			sub[id] = st
		}
	}
	if len(sub) == 0 {
		return nil
	}

	locs, pending := app.Services.Positions.KoboLocations(ctx, userID, sub)
	for id, loc := range locs {
		withLoc := *states[id]
		withLoc.KoboLocation = loc
		states[id] = &withLoc
	}
	return pending
}

// backfillKoboPosition stores a PUT's neutral position once its span map is
// built, unless a newer write replaced the bookmark meanwhile.
func (app *Books) backfillKoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
) {
	bgCtx := context.WithoutCancel(ctx)
	go app.Services.Positions.BackfillKoboPosition(bgCtx, userID, bookID, loc,
		func(ctx context.Context, pos models.ReadingPosition) error {
			return app.Services.Books.SetKoboPosition(ctx, userID, bookID, loc, pos)
		})
}
