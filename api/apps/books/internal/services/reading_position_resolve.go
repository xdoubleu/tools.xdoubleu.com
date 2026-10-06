package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// kepubRef is a ready KEPUB and the file its span map translates against.
type kepubRef struct {
	key                 spanMapKey
	kepubKey, sourceKey string
	// pdf: converted from a PDF, so the map needs only the KEPUB.
	pdf bool
}

// KoboLocations returns, for the states whose Kobo location changes, the one
// to send (nil: none). A state without a bookmark gets its neutral position
// translated. A replaced book's device still holds the old KEPUB, and a
// PDF-sourced KEPUB's regeneration can renumber its spans, so its bookmark is
// re-derived from the position, or dropped. pending holds the books whose
// span map was still building at the budget.
func (s *PositionService) KoboLocations(
	ctx context.Context,
	userID string,
	states map[uuid.UUID]*models.BookReadingState,
	replaced map[uuid.UUID]bool,
) (map[uuid.UUID]*models.KoboLocation, map[uuid.UUID]bool) {
	timer := time.NewTimer(s.budget)
	defer timer.Stop()

	ids := make([]uuid.UUID, 0, len(states))
	for id, st := range states {
		if st != nil && (st.KoboLocation == nil && st.Position != nil ||
			st.KoboLocation != nil && replaced[id]) {
			ids = append(ids, id)
		}
	}
	locs := make(map[uuid.UUID]*models.KoboLocation)
	need := make(map[uuid.UUID]kepubRef)
	for id, ref := range s.resolveAll(ctx, userID, ids) {
		st := states[id]
		if st.KoboLocation != nil {
			if !ref.pdf {
				continue
			}
			locs[id] = nil
		}
		if st.Position != nil && (ref.pdf || st.Position.Page == 0) {
			need[id] = ref
		}
	}

	maps, pending := s.mapsOf(ctx, timer, need)
	for id, m := range maps {
		if loc := m.location(*states[id].Position); loc != nil {
			locs[id] = loc
		}
	}
	return locs, pending
}

// ReaderPosition is pos in both of the web reader's forms when bookID's KEPUB
// was converted from a PDF and its span map is ready within the budget,
// otherwise pos unchanged.
func (s *PositionService) ReaderPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	pos models.ReadingPosition,
) models.ReadingPosition {
	timer := time.NewTimer(s.budget)
	defer timer.Stop()

	refs := s.resolveAll(ctx, userID, []uuid.UUID{bookID})
	if !refs[bookID].pdf {
		return pos
	}
	maps, _ := s.mapsOf(ctx, timer, refs)
	if m := maps[bookID]; m != nil {
		if both := m.readerPosition(pos); both != nil {
			return *both
		}
	}
	return pos
}

// resolveAll resolves the books that have something to translate against.
func (s *PositionService) resolveAll(
	ctx context.Context,
	userID string,
	bookIDs []uuid.UUID,
) map[uuid.UUID]kepubRef {
	refs := make(map[uuid.UUID]kepubRef, len(bookIDs))
	for _, bookID := range bookIDs {
		if ref, ok := s.resolve(ctx, userID, bookID); ok {
			refs[bookID] = ref
		}
	}
	return refs
}

// resolve finds the user's ready KEPUB of bookID and its EPUB or PDF source;
// ok is false when there's nothing to translate against.
func (s *PositionService) resolve(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (kepubRef, bool) {
	kepub, err := s.files.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatKEPUB,
	)
	if err == nil && kepub.Status == models.FileStatusReady &&
		kepub.SourceFileID != nil {
		var source *models.BookFile
		source, err = s.files.GetByID(ctx, *kepub.SourceFileID)
		if err == nil && (source.Format == models.FileFormatEPUB ||
			source.Format == models.FileFormatPDF) {
			return kepubRef{
				key: spanMapKey{
					kepubID: kepub.ID, sourceID: source.ID, version: kepub.ConverterVersion,
				},
				kepubKey:  kepub.StorageKey,
				sourceKey: source.StorageKey,
				pdf:       source.Format == models.FileFormatPDF,
			}, true
		}
	}
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		s.logger.WarnContext(ctx, "kepub span map lookup failed",
			"book_id", bookID, "err", err)
	}
	//nolint:exhaustruct // not found
	return kepubRef{}, false
}
