package kobo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// ErrBadStateBody marks a state PUT whose body isn't the expected JSON.
var ErrBadStateBody = errors.New("kobo: bad reading state body")

// handleNotFound maps a lookup error to (found, error): not-found is a plain
// false.
func handleNotFound(err error) (bool, error) {
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, database.ErrResourceNotFound):
		return false, nil
	default:
		return false, err
	}
}

// DeviceLacks reports whether a device holding held lacks state: the
// book is new to it, or the state changed since.
func DeviceLacks(
	held map[uuid.UUID]time.Time,
	bookID uuid.UUID,
	state *models.BookReadingState,
) bool {
	prev, onDevice := held[bookID]
	return !onDevice || (state != nil && state.UpdatedAt.After(prev))
}

// LibraryBookID returns the library book a state id names; false means a Kobo
// store book, which the store serves.
func (p *Protocol) LibraryBookID(
	ctx context.Context,
	userID, id string,
) (uuid.UUID, bool, error) {
	bookID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, false, nil //nolint:nilerr // a non-UUID id is a store book
	}
	_, err = p.d.Library.GetUserBook(ctx, userID, bookID)
	found, err := handleNotFound(err)
	if err != nil || !found {
		return uuid.Nil, false, err
	}
	return bookID, true, nil
}

// GetState is a library book's reading state with its KoboSpan location; the
// state is zero-valued when nothing is stored.
func (p *Protocol) GetState(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*ReadingState, error) {
	state, err := p.d.Library.GetReadingState(ctx, userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}
	if state != nil {
		states := map[uuid.UUID]*models.BookReadingState{bookID: state}
		p.withSpans(ctx, userID, []uuid.UUID{bookID}, states, nil)
		state = states[bookID]
	}
	return buildState(bookID.String(), state), nil
}

// stateBody is a device's state PUT. Devices send a plural ReadingStates array
// and whole-book ProgressPercent 0-100.
type stateBody struct {
	ReadingStates []struct {
		CurrentBookmark struct {
			LastModified    string          `json:"LastModified"`
			ProgressPercent int             `json:"ProgressPercent"`
			Location        json.RawMessage `json:"Location"`
		} `json:"CurrentBookmark"`
		StatusInfo struct {
			Status string `json:"Status"`
		} `json:"StatusInfo"`
	} `json:"ReadingStates"`
}

// PutState stores a device's reading position for a library book and returns
// the ack in the shape of calibre-web's HandleStateRequest.
func (p *Protocol) PutState(
	ctx context.Context,
	userID, deviceID string,
	bookID uuid.UUID,
	body io.Reader,
) (StateAck, error) {
	var in stateBody
	if err := json.NewDecoder(body).Decode(&in); err != nil {
		return StateAck{}, ErrBadStateBody
	}

	ack := StateAck{
		RequestResult: resultSuccess,
		UpdateResults: []updateResult{},
	}
	if len(in.ReadingStates) == 0 {
		return ack, nil
	}

	rs := in.ReadingStates[len(in.ReadingStates)-1]
	state := models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
		UserID:  userID,
		BookID:  bookID,
		Percent: rs.CurrentBookmark.ProgressPercent,
		ReadAt:  parseTime(rs.CurrentBookmark.LastModified),
	}
	backfill := p.putLocation(
		ctx, &state, rs.CurrentBookmark.Location, rs.StatusInfo.Status,
	)

	if err := p.d.Library.UpdateKoboReadingProgress(ctx, deviceID, state); err != nil {
		return StateAck{}, err
	}
	if backfill {
		p.backfillPosition(ctx, userID, bookID, *state.KoboLocation)
	}
	ack.UpdateResults = append(ack.UpdateResults, updateResult{
		EntitlementId:         bookID.String(),
		CurrentBookmarkResult: result{Result: resultSuccess},
		StatisticsResult:      result{Result: "Ignored"},
		StatusInfoResult:      result{Result: resultSuccess},
	})
	return ack, nil
}

// putLocation sets a PUT state's bookmark and its translated position. A
// finished book's Source is the first resource, so it's kept as Value only.
// It reports whether the position should be backfilled once translatable.
func (p *Protocol) putLocation(
	ctx context.Context,
	state *models.BookReadingState,
	raw json.RawMessage,
	status string,
) bool {
	loc := parseLocation(raw)
	if loc == nil {
		return false
	}
	state.Location = &loc.Value
	if loc.Source == "" || status == statusFinished {
		return false
	}
	state.KoboLocation = loc
	book, err := p.d.Library.GetKoboSyncBook(ctx, state.UserID, state.BookID)
	outdated := err == nil && isReplace(book)
	var backfill bool
	state.Position, backfill = p.d.Positions.KoboPosition(
		ctx, state.UserID, state.BookID, *loc, outdated,
	)
	return backfill
}

// syncSpans translates the states deviceID lacks (it ignores the state in
// a repeated NewEntitlement) and those of regenerated KEPUBs, returning what
// it holds and the pending books.
func (p *Protocol) syncSpans(
	ctx context.Context,
	userID, deviceID string,
	books []models.KoboSyncBook,
	states map[uuid.UUID]*models.BookReadingState,
) (map[uuid.UUID]time.Time, map[uuid.UUID]bool, error) {
	held, err := p.d.Library.ListKoboDeviceHeld(ctx, deviceID)
	if err != nil {
		return nil, nil, err
	}
	var lacking []uuid.UUID
	replaced := make(map[uuid.UUID]bool)
	for _, b := range books {
		replaced[b.BookID] = isReplace(b)
		if replaced[b.BookID] || DeviceLacks(held, b.BookID, states[b.BookID]) {
			lacking = append(lacking, b.BookID)
		}
	}
	return held, p.withSpans(ctx, userID, lacking, states, replaced), nil
}

// withSpans replaces each KEPUB book's state in states with one carrying
// the KoboSpan location to send (PositionService.KoboLocations). A replaced
// book's stale bookmark is also cleared from the row, so later reads translate
// the position, and the bumped state reaches the device again on a later sync
// even if this one went out without a location. pending holds books whose
// span map is still building.
func (p *Protocol) withSpans(
	ctx context.Context,
	userID string,
	kepubBooks []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
	replaced map[uuid.UUID]bool,
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

	locs, pending := p.d.Positions.KoboLocations(ctx, userID, sub, replaced)
	for id, loc := range locs {
		withLoc := *states[id]
		if stale := withLoc.KoboLocation; stale != nil {
			if err := p.d.Library.ClearKoboLocation(ctx, userID, id, *stale); err != nil {
				p.d.Logger.WarnContext(ctx, "clearing a stale kobo location failed",
					"book_id", id, "err", err)
			}
		}
		withLoc.KoboLocation = loc
		states[id] = &withLoc
	}
	return pending
}

// backfillPosition stores a PUT's neutral position once its span map is
// built, unless a newer write replaced the bookmark meanwhile.
func (p *Protocol) backfillPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
) {
	p.d.Background.Go(ctx, "kobo-position-backfill", func(ctx context.Context) {
		p.d.Positions.BackfillKoboPosition(ctx, userID, bookID, loc,
			func(ctx context.Context, pos models.ReadingPosition) error {
				return p.d.Library.SetKoboPosition(ctx, userID, bookID, loc, pos)
			})
	})
}

// changedReadingStates builds ChangedReadingState entries for the states
// deviceID lacks, leaving pending books for the next sync.
func (p *Protocol) changedReadingStates(
	ctx context.Context,
	deviceID string,
	books []models.KoboSyncBook,
	stateByBook map[uuid.UUID]*models.BookReadingState,
	held map[uuid.UUID]time.Time,
	pending map[uuid.UUID]bool,
) ([]json.RawMessage, error) {
	bookIDs := make([]uuid.UUID, len(books))
	for i, b := range books {
		bookIDs[i] = b.BookID
	}
	changed, err := p.d.Library.SyncKoboDeviceReadingStates(
		ctx, deviceID, bookIDs, stateByBook, held, pending,
	)
	if err != nil {
		return nil, err
	}

	entries := make([]json.RawMessage, len(changed))
	for i, state := range changed {
		var entry changedReadingState
		entry.ChangedReadingState.ReadingState = buildState(
			state.BookID.String(), state,
		)
		entries[i], _ = json.Marshal(entry)
	}
	return entries, nil
}

// parseLocation reads CurrentBookmark.Location as {Source,Type,Value},
// falling back to a bare string (Value only). An empty Value is no location.
func parseLocation(raw json.RawMessage) *models.KoboLocation {
	if len(raw) == 0 {
		return nil
	}
	var loc models.KoboLocation
	if err := json.Unmarshal(raw, &loc); err == nil && loc.Value != "" {
		return &loc
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return &models.KoboLocation{Source: "", Type: "", Value: s}
	}
	return nil
}

// parseTime parses a device timestamp, nil when absent or malformed.
func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}

// statusForPercent derives StatusInfo.Status from percent alone.
func statusForPercent(percent int) string {
	switch {
	case percent >= models.MaxProgressPercent:
		return statusFinished
	case percent > 0:
		return statusReading
	default:
		return "ReadyToRead"
	}
}

// buildState renders a reading state. LastModified is the server's
// updated_at, so every server change looks newer to the device.
func buildState(id string, state *models.BookReadingState) *ReadingState {
	ts := epoch
	percent := 0
	var loc *models.KoboLocation
	if state != nil {
		ts = state.UpdatedAt.UTC().Format(time.RFC3339)
		percent = state.Percent
		loc = state.KoboLocation
	}
	return &ReadingState{
		CurrentBookmark: bookmark{
			LastModified:                 ts,
			ProgressPercent:              percent,
			ContentSourceProgressPercent: percent,
			Location:                     loc,
		},
		EntitlementId:     id,
		LastModified:      ts,
		PriorityTimestamp: ts,
		StatusInfo: statusInfo{
			LastModified: ts,
			Status:       statusForPercent(percent),
			TimestampId:  id,
		},
	}
}
