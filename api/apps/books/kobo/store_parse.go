package kobo

import (
	"encoding/json"
	"math"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// MaxStateBody caps a buffered reading-state PUT.
const MaxStateBody = 1 << 20

// storeItem is the part of an upstream sync item read for store books.
type storeItem struct {
	NewEntitlement      *storeEntry `json:"NewEntitlement"`
	ChangedEntitlement  *storeEntry `json:"ChangedEntitlement"`
	ChangedReadingState *struct {
		ReadingState *storeState `json:"ReadingState"`
	} `json:"ChangedReadingState"`
}

type storeEntry struct {
	BookEntitlement struct {
		Accessibility string            `json:"Accessibility"`
		ActivePeriod  map[string]string `json:"ActivePeriod"`
		//nolint:revive // Kobo protocol field name
		Id             string `json:"Id"`
		IsRemoved      bool   `json:"IsRemoved"`
		OriginCategory string `json:"OriginCategory"`
	} `json:"BookEntitlement"`
	BookMetadata storeMetadata `json:"BookMetadata"`
	ReadingState *storeState   `json:"ReadingState"`
}

type storeMetadata struct {
	Title            string          `json:"Title"`
	Isbn             string          `json:"Isbn"`
	Contributors     json.RawMessage `json:"Contributors"`
	ContributorRoles []struct {
		Name string `json:"Name"`
		Role string `json:"Role"`
	} `json:"ContributorRoles"`
}

type storeState struct {
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
func (m storeMetadata) toStoreBook(id string, owned *bool) models.KoboStoreBook {
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
func (m storeMetadata) authors() []string {
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

func (s *storeState) toReading(fallbackID string) models.KoboStoreReading {
	id := s.EntitlementId
	if id == "" {
		id = fallbackID
	}
	status := s.StatusInfo.Status
	return models.KoboStoreReading{
		EntitlementID: id,
		Percent:       storePercent(s.CurrentBookmark.ProgressPercent, status),
		Finished:      status == statusFinished,
		ReadAt:        parseTime(s.CurrentBookmark.LastModified),
	}
}

// storePercent rounds the store's fractional percent; any positive
// progress, or an opened book at 0, is at least 1, as the device shows it.
func storePercent(p float64, status string) int {
	if p > 0 && p < 1 || p == 0 && status == statusReading {
		return 1
	}
	return int(math.Round(p))
}

// storeOwned reports whether an entitlement is a purchase: full access,
// not removed, and not time-limited or subscription-based.
func storeOwned(e *storeEntry) bool {
	ent := e.BookEntitlement
	if ent.Accessibility != accessFull || ent.IsRemoved ||
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

// parseStoreItems extracts store books and reading states from upstream
// sync items, skipping item kinds it doesn't read.
func parseStoreItems(
	items []json.RawMessage,
) ([]models.KoboStoreBook, []models.KoboStoreReading) {
	var storeBooks []models.KoboStoreBook
	var readings []models.KoboStoreReading
	for _, raw := range items {
		var item storeItem
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		if crs := item.ChangedReadingState; crs != nil && crs.ReadingState != nil {
			if rd := crs.ReadingState.toReading(""); rd.EntitlementID != "" {
				readings = append(readings, rd)
			}
		}
		for _, e := range []*storeEntry{item.NewEntitlement, item.ChangedEntitlement} {
			if e == nil || e.BookEntitlement.Id == "" {
				continue
			}
			owned := storeOwned(e)
			id := e.BookEntitlement.Id
			storeBooks = append(storeBooks, e.BookMetadata.toStoreBook(id, &owned))
			if e.ReadingState != nil {
				readings = append(readings, e.ReadingState.toReading(id))
			}
		}
	}
	return storeBooks, readings
}
