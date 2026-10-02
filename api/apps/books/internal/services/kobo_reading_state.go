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

// SyncKoboDeviceReadingStates returns the states that changed since deviceID
// last held them, for books it already has, then records every book's current
// state as held. A book new to the device gets its state with its entitlement.
func (s *BookService) SyncKoboDeviceReadingStates(
	ctx context.Context,
	deviceID string,
	bookIDs []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
) ([]*models.BookReadingState, error) {
	held, err := s.readingState.ListKoboDeviceHeld(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	var changed []*models.BookReadingState
	var markIDs []uuid.UUID
	var markAts []time.Time
	for _, bookID := range bookIDs {
		state := states[bookID]
		at := time.Unix(0, 0).UTC()
		if state != nil {
			at = state.UpdatedAt
		}
		prev, onDevice := held[bookID]
		if onDevice && !at.After(prev) {
			continue
		}
		if onDevice {
			changed = append(changed, state)
		}
		markIDs = append(markIDs, bookID)
		markAts = append(markAts, at)
	}

	if len(markIDs) > 0 {
		if err = s.readingState.MarkKoboDeviceHeld(
			ctx, deviceID, markIDs, markAts,
		); err != nil {
			return nil, err
		}
	}
	return changed, nil
}
