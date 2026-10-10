package books

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// koboMaxStateBody caps a buffered reading-state PUT.
const koboMaxStateBody = 1 << 20

// koboStoreItem is the part of an upstream sync item read for store books.
type koboStoreItem struct {
	NewEntitlement      *koboStoreEntry `json:"NewEntitlement"`
	ChangedEntitlement  *koboStoreEntry `json:"ChangedEntitlement"`
	ChangedReadingState *struct {
		ReadingState *koboStoreState `json:"ReadingState"`
	} `json:"ChangedReadingState"`
}

type koboStoreEntry struct {
	BookEntitlement struct {
		Accessibility string            `json:"Accessibility"`
		ActivePeriod  map[string]string `json:"ActivePeriod"`
		//nolint:revive // Kobo protocol field name
		Id             string `json:"Id"`
		IsRemoved      bool   `json:"IsRemoved"`
		OriginCategory string `json:"OriginCategory"`
	} `json:"BookEntitlement"`
	BookMetadata koboStoreMetadata `json:"BookMetadata"`
	ReadingState *koboStoreState   `json:"ReadingState"`
}

type koboStoreMetadata struct {
	Title            string          `json:"Title"`
	Isbn             string          `json:"Isbn"`
	Contributors     json.RawMessage `json:"Contributors"`
	ContributorRoles []struct {
		Name string `json:"Name"`
		Role string `json:"Role"`
	} `json:"ContributorRoles"`
}

type koboStoreState struct {
	//nolint:revive // Kobo protocol field name
	EntitlementId   string `json:"EntitlementId"`
	CurrentBookmark struct {
		LastModified    string  `json:"LastModified"`
		ProgressPercent float64 `json:"ProgressPercent"`
	} `json:"CurrentBookmark"`
	StatusInfo struct {
		Status string `json:"Status"`
	} `json:"StatusInfo"`
}

// toStoreBook converts store metadata; owned is nil when unknown.
func (m koboStoreMetadata) toStoreBook(id string, owned *bool) models.KoboStoreBook {
	var isbn *string
	if m.Isbn != "" {
		isbn = &m.Isbn
	}
	return models.KoboStoreBook{
		EntitlementID: id,
		ISBN13:        isbn,
		Title:         m.Title,
		Authors:       m.authors(),
		Owned:         owned,
	}
}

// authors reads ContributorRoles' authors, else a Contributors string list.
func (m koboStoreMetadata) authors() []string {
	var out []string
	for _, c := range m.ContributorRoles {
		if c.Name != "" && (c.Role == "" || strings.EqualFold(c.Role, "Author")) {
			out = append(out, c.Name)
		}
	}
	if len(out) > 0 {
		return out
	}
	var names []string
	if json.Unmarshal(m.Contributors, &names) == nil {
		return names
	}
	return nil
}

func (s *koboStoreState) toReading(fallbackID string) models.KoboStoreReading {
	id := s.EntitlementId
	if id == "" {
		id = fallbackID
	}
	return models.KoboStoreReading{
		EntitlementID: id,
		Percent:       int(math.Round(s.CurrentBookmark.ProgressPercent)),
		Finished:      s.StatusInfo.Status == koboStatusFinished,
		ReadAt:        parseKoboTime(s.CurrentBookmark.LastModified),
	}
}

// koboStoreOwned reports whether an entitlement is a purchase: full access,
// not removed, and not time-limited or subscription-based.
func koboStoreOwned(e *koboStoreEntry) bool {
	ent := e.BookEntitlement
	if ent.Accessibility != koboAccessFull || ent.IsRemoved ||
		ent.ActivePeriod["To"] != "" {
		return false
	}
	origin := strings.ToLower(ent.OriginCategory)
	for _, kind := range []string{"subscription", "plus", "preview", "sample", "loan"} {
		if strings.Contains(origin, kind) {
			return false
		}
	}
	return true
}

// parseKoboStoreItems extracts store books and reading states from upstream
// sync items, skipping item kinds it doesn't read.
func parseKoboStoreItems(
	items []json.RawMessage,
) ([]models.KoboStoreBook, []models.KoboStoreReading) {
	var storeBooks []models.KoboStoreBook
	var readings []models.KoboStoreReading
	for _, raw := range items {
		var item koboStoreItem
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		if crs := item.ChangedReadingState; crs != nil && crs.ReadingState != nil {
			if rd := crs.ReadingState.toReading(""); rd.EntitlementID != "" {
				readings = append(readings, rd)
			}
		}
		for _, e := range []*koboStoreEntry{item.NewEntitlement, item.ChangedEntitlement} {
			if e == nil || e.BookEntitlement.Id == "" {
				continue
			}
			owned := koboStoreOwned(e)
			id := e.BookEntitlement.Id
			storeBooks = append(storeBooks, e.BookMetadata.toStoreBook(id, &owned))
			if e.ReadingState != nil {
				readings = append(readings, e.ReadingState.toReading(id))
			}
		}
	}
	return storeBooks, readings
}

// recordKoboStoreSync links the store books of an upstream sync page to the
// library and mirrors their reading states. Runs detached from the request.
func (app *Books) recordKoboStoreSync(
	ctx context.Context,
	userID string,
	items []json.RawMessage,
) {
	storeBooks, readings := parseKoboStoreItems(items)
	if err := app.Services.Books.LinkKoboStoreBooks(ctx, userID, storeBooks); err != nil {
		app.Logger.WarnContext(ctx, "linking kobo store books failed", "err", err)
		return
	}
	for _, rd := range readings {
		if _, err := app.Services.Books.RecordKoboStoreReading(ctx, userID, rd); err != nil {
			app.Logger.WarnContext(ctx, "mirroring kobo store reading state failed",
				"entitlement_id", rd.EntitlementID, "err", err)
		}
	}
}

// koboLibraryBookID returns the library book a state id names; false
// means a Kobo store book, which the store serves.
func (app *Books) koboLibraryBookID(
	ctx context.Context,
	userID, id string,
) (uuid.UUID, bool, error) {
	bookID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, false, nil //nolint:nilerr // a non-UUID id is a store book
	}
	_, err = app.Services.Books.GetUserBook(ctx, userID, bookID)
	switch {
	case err == nil:
		return bookID, true, nil
	case errors.Is(err, database.ErrResourceNotFound):
		return uuid.Nil, false, nil
	default:
		return uuid.Nil, false, err
	}
}

// koboStorePutState proxies a store book's state PUT upstream, then mirrors
// it onto the library in the background.
func (app *Books) koboStorePutState(
	w http.ResponseWriter,
	r *http.Request,
	userID, id string,
) {
	body, err := io.ReadAll(io.LimitReader(r.Body, koboMaxStateBody))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	app.koboProxyHandler(w, r)

	go app.mirrorKoboStoreState(
		context.WithoutCancel(r.Context()), userID, id, r.Header.Clone(), body,
	)
}

// mirrorKoboStoreState records a store book's PUT state on its library match,
// first learning the book from the store's metadata if it's new to us.
func (app *Books) mirrorKoboStoreState(
	ctx context.Context,
	userID, id string,
	hdr http.Header,
	body []byte,
) {
	var put struct {
		ReadingStates []koboStoreState `json:"ReadingStates"`
	}
	if json.Unmarshal(body, &put) != nil || len(put.ReadingStates) == 0 {
		return
	}
	reading := put.ReadingStates[len(put.ReadingStates)-1].toReading(id)
	reading.EntitlementID = id

	found, err := app.Services.Books.RecordKoboStoreReading(ctx, userID, reading)
	if err == nil && !found {
		var sb *models.KoboStoreBook
		if sb, err = app.koboFetchStoreMetadata(ctx, hdr, id); err == nil {
			err = app.Services.Books.LinkKoboStoreBooks(
				ctx, userID, []models.KoboStoreBook{*sb},
			)
		}
		if err == nil {
			_, err = app.Services.Books.RecordKoboStoreReading(ctx, userID, reading)
		}
	}
	if err != nil {
		app.Logger.WarnContext(ctx, "mirroring kobo store reading state failed",
			"entitlement_id", id, "err", err)
	}
}

// koboFetchStoreMetadata reads a store book's metadata from the Kobo store.
// Ownership is unknown from metadata alone.
func (app *Books) koboFetchStoreMetadata(
	ctx context.Context,
	src http.Header,
	id string,
) (*models.KoboStoreBook, error) {
	targetURL := app.clients.KoboStoreBaseURL + "/v1/library/" +
		url.PathEscape(id) + "/metadata"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = src.Clone()
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Content-Length")
	req.Header.Del("Content-Type")
	koboForwardAuth(req.Header, src)

	resp, err := koboUpstreamClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(koboUpstreamNote(resp.StatusCode, src))
	}

	var metas []koboStoreMetadata
	if err = json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		return nil, err
	}
	if len(metas) == 0 {
		return nil, errors.New("upstream: empty metadata")
	}
	sb := metas[0].toStoreBook(id, nil)
	return &sb, nil
}
