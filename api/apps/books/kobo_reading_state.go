package books

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

type koboReadingState struct {
	CurrentBookmark koboBookmark `json:"CurrentBookmark"`
	//nolint:revive // Kobo protocol field name
	EntitlementId     string         `json:"EntitlementId"`
	LastModified      string         `json:"LastModified"`
	PriorityTimestamp string         `json:"PriorityTimestamp"`
	StatusInfo        koboStatusInfo `json:"StatusInfo"`
}

type koboBookmark struct {
	LastModified    string `json:"LastModified"`
	ProgressPercent int    `json:"ProgressPercent"`
	// ContentSourceProgressPercent is within-chapter on devices; we mirror the
	// whole-book percent.
	ContentSourceProgressPercent int                  `json:"ContentSourceProgressPercent"`
	Location                     *models.KoboLocation `json:"Location,omitempty"`
}

type koboStatusInfo struct {
	LastModified string `json:"LastModified"`
	Status       string `json:"Status"`
	//nolint:revive // Kobo protocol field name
	TimestampId string `json:"TimestampId"`
}

// koboChangedReadingState pushes a new position for a book already on the
// device; it ignores ReadingState in a repeated NewEntitlement.
type koboChangedReadingState struct {
	ChangedReadingState struct {
		ReadingState *koboReadingState `json:"ReadingState"`
	} `json:"ChangedReadingState"`
}

// koboStateAck answers a state PUT. Without a per-section Success the device
// keeps its local copy authoritative and ignores server positions.
type koboStateAck struct {
	RequestResult string             `json:"RequestResult"`
	UpdateResults []koboUpdateResult `json:"UpdateResults"`
}

type koboUpdateResult struct {
	//nolint:revive // Kobo protocol field name
	EntitlementId         string     `json:"EntitlementId"`
	CurrentBookmarkResult koboResult `json:"CurrentBookmarkResult"`
	StatisticsResult      koboResult `json:"StatisticsResult"`
	StatusInfoResult      koboResult `json:"StatusInfoResult"`
}

type koboResult struct {
	Result string `json:"Result"`
}

// koboStatusFinished is the StatusInfo.Status of a finished book, whose
// bookmark Source is the first resource rather than the reading position.
const koboStatusFinished = "Finished"

const koboResultSuccess = "Success"

// koboChangedReadingStates builds ChangedReadingState entries for the states
// deviceID lacks, leaving pending books for the next sync.
func (app *Books) koboChangedReadingStates(
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
	changed, err := app.Services.Books.SyncKoboDeviceReadingStates(
		ctx, deviceID, bookIDs, stateByBook, held, pending,
	)
	if err != nil {
		return nil, err
	}

	entries := make([]json.RawMessage, len(changed))
	for i, state := range changed {
		var entry koboChangedReadingState
		entry.ChangedReadingState.ReadingState = buildKoboState(
			state.BookID.String(), state,
		)
		entries[i], _ = json.Marshal(entry)
	}
	return entries, nil
}

// koboGetStateHandler handles GET /v1/library/{revisionId}/state; a Kobo store
// book's state is the store's.
func (app *Books) koboGetStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, isLibrary, err := app.koboLibraryBookID(
		r.Context(), userID, r.PathValue("revisionId"),
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isLibrary {
		app.koboProxyHandler(w, r)
		return
	}

	state, err := app.Services.Books.GetReadingState(r.Context(), userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if state != nil {
		format, fmtErr := app.Services.Books.GetKoboFileFormat(
			r.Context(), userID, bookID,
		)
		if fmtErr == nil && format == models.FileFormatKEPUB {
			states := map[uuid.UUID]*models.BookReadingState{bookID: state}
			app.koboWithSpans(r.Context(), userID, []uuid.UUID{bookID}, states, nil)
			state = states[bookID]
		}
	}

	koboWriteJSON(w, buildKoboState(bookID.String(), state))
}

// koboPutStateHandler handles PUT /v1/library/{revisionId}/state, answering
// with the ack shape of calibre-web's HandleStateRequest. A Kobo store book's
// state goes to the store (koboStorePutState).
func (app *Books) koboPutStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, deviceID, ok := app.koboAuthDevice(w, r)
	if !ok {
		return
	}

	id := r.PathValue("revisionId")
	bookID, isLibrary, err := app.koboLibraryBookID(r.Context(), userID, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isLibrary {
		app.koboStorePutState(w, r, userID, id)
		return
	}

	// Devices send a plural ReadingStates array and whole-book ProgressPercent
	// 0-100.
	var body struct {
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
	if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ack := koboStateAck{
		RequestResult: koboResultSuccess,
		UpdateResults: []koboUpdateResult{},
	}
	if len(body.ReadingStates) > 0 {
		rs := body.ReadingStates[len(body.ReadingStates)-1]
		state := models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:  userID,
			BookID:  bookID,
			Percent: rs.CurrentBookmark.ProgressPercent,
			ReadAt:  parseKoboTime(rs.CurrentBookmark.LastModified),
		}
		backfill := app.koboPutLocation(
			r.Context(), &state, rs.CurrentBookmark.Location, rs.StatusInfo.Status,
		)

		if err = app.Services.Books.UpdateKoboReadingProgress(
			r.Context(), deviceID, state,
		); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if backfill {
			app.backfillKoboPosition(r.Context(), userID, bookID, *state.KoboLocation)
		}
		ack.UpdateResults = append(ack.UpdateResults, koboUpdateResult{
			EntitlementId:         bookID.String(),
			CurrentBookmarkResult: koboResult{Result: koboResultSuccess},
			StatisticsResult:      koboResult{Result: "Ignored"},
			StatusInfoResult:      koboResult{Result: koboResultSuccess},
		})
	}

	koboWriteJSON(w, ack)
}

// parseKoboLocation reads CurrentBookmark.Location as {Source,Type,Value},
// falling back to a bare string (Value only). An empty Value is no location.
func parseKoboLocation(raw json.RawMessage) *models.KoboLocation {
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

// parseKoboTime parses a device timestamp, nil when absent or malformed.
func parseKoboTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}

// koboStatusForPercent derives StatusInfo.Status from percent alone.
func koboStatusForPercent(percent int) string {
	switch {
	case percent >= models.MaxProgressPercent:
		return koboStatusFinished
	case percent > 0:
		return "Reading"
	default:
		return "ReadyToRead"
	}
}

// buildKoboState renders a reading state. LastModified is the server's
// updated_at, so every server change looks newer to the device.
func buildKoboState(id string, state *models.BookReadingState) *koboReadingState {
	ts := koboEpoch
	percent := 0
	var loc *models.KoboLocation
	if state != nil {
		ts = state.UpdatedAt.UTC().Format(time.RFC3339)
		percent = state.Percent
		loc = state.KoboLocation
	}
	return &koboReadingState{
		CurrentBookmark: koboBookmark{
			LastModified:                 ts,
			ProgressPercent:              percent,
			ContentSourceProgressPercent: percent,
			Location:                     loc,
		},
		EntitlementId:     id,
		LastModified:      ts,
		PriorityTimestamp: ts,
		StatusInfo: koboStatusInfo{
			LastModified: ts,
			Status:       koboStatusForPercent(percent),
			TimestampId:  id,
		},
	}
}
