package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// UpdateKoboReadingProgress stores a Kobo device's position. When applied, the
// new state is recorded as held by deviceID so its next sync doesn't echo it.
func (s *BookService) UpdateKoboReadingProgress(
	ctx context.Context,
	deviceID string,
	state models.BookReadingState,
) error {
	state.Source = models.ReadingSourceKobo
	updatedAt, err := s.upsertReadingProgress(ctx, state)
	if err != nil || updatedAt == nil {
		return err
	}
	return s.readingState.MarkKoboDeviceHeld(
		ctx, deviceID, []uuid.UUID{state.BookID}, []time.Time{*updatedAt},
	)
}

// ListKoboDeviceHeld returns, per book, the state updated_at deviceID holds.
func (s *BookService) ListKoboDeviceHeld(
	ctx context.Context,
	deviceID string,
) (map[uuid.UUID]time.Time, error) {
	return s.readingState.ListKoboDeviceHeld(ctx, deviceID)
}

// KoboDeviceLacks reports whether a device holding held lacks state: the
// book is new to it, or the state changed since.
func KoboDeviceLacks(
	held map[uuid.UUID]time.Time,
	bookID uuid.UUID,
	state *models.BookReadingState,
) bool {
	prev, onDevice := held[bookID]
	return !onDevice || (state != nil && state.UpdatedAt.After(prev))
}

// SyncKoboDeviceReadingStates returns the states that changed since deviceID
// last held them (held, from ListKoboDeviceHeld), for books it already has,
// then records every book's current state as held. A book new to the device
// gets its state with its entitlement. A pending book (its span location still
// being computed) is left for the next sync: not returned, and held at the
// epoch if new to the device.
func (s *BookService) SyncKoboDeviceReadingStates(
	ctx context.Context,
	deviceID string,
	bookIDs []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
	held map[uuid.UUID]time.Time,
	pending map[uuid.UUID]bool,
) ([]*models.BookReadingState, error) {
	var changed []*models.BookReadingState
	var markIDs []uuid.UUID
	var markAts []time.Time
	epoch := time.Unix(0, 0).UTC()
	for _, bookID := range bookIDs {
		state := states[bookID]
		at := epoch
		if state != nil {
			at = state.UpdatedAt
		}
		if !KoboDeviceLacks(held, bookID, state) {
			continue
		}
		_, onDevice := held[bookID]
		if pending[bookID] {
			if !onDevice {
				markIDs = append(markIDs, bookID)
				markAts = append(markAts, epoch)
			}
			continue
		}
		if onDevice {
			changed = append(changed, state)
		}
		markIDs = append(markIDs, bookID)
		markAts = append(markAts, at)
	}

	if len(markIDs) > 0 {
		if err := s.readingState.MarkKoboDeviceHeld(
			ctx, deviceID, markIDs, markAts,
		); err != nil {
			return nil, err
		}
	}
	return changed, nil
}

// ClearKoboLocation drops a Kobo bookmark that no longer fits the KEPUB the
// device gets; a no-op once the bookmark changed.
func (s *BookService) ClearKoboLocation(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
) error {
	return s.readingState.ClearKoboLocation(ctx, userID, bookID, loc)
}

// SetKoboPosition fills in the neutral position of a stored Kobo bookmark that
// was saved without one; a no-op once the bookmark changed.
func (s *BookService) SetKoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
	pos models.ReadingPosition,
) error {
	return s.readingState.SetPositionForKoboLocation(ctx, userID, bookID, loc, pos)
}
