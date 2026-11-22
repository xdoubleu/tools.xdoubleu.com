package protomap

import (
	"strings"
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/kobo"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// KoboLogEntryProto converts a captured entry to its proto form. Captured
// bodies come from arbitrary Kobo/upstream traffic (including binary/gzip
// responses proxied verbatim) and the 64KB capture cap can slice a body
// mid-rune, so they are not guaranteed valid UTF-8. proto3 string fields must
// be valid UTF-8 or marshaling fails, so sanitize here — the single point
// every captured entry passes through on its way into the response.
func KoboLogEntryProto(e kobo.LogEntry) *booksv1.KoboLogEntry {
	return &booksv1.KoboLogEntry{
		Time:         e.Time.Format(time.RFC3339),
		Method:       e.Method,
		Path:         e.Path,
		Query:        e.Query,
		RequestBody:  strings.ToValidUTF8(e.RequestBody, "�"),
		Status:       int32(e.Status), //nolint:gosec // HTTP status fits int32
		ResponseBody: strings.ToValidUTF8(e.ResponseBody, "�"),
		Notes:        e.Notes,
		// Header values are proxied from the device and upstream verbatim.
		RequestHeaders:  validUTF8Map(e.RequestHeaders),
		ResponseHeaders: validUTF8Map(e.ResponseHeaders),
		UpstreamHeaders: validUTF8Map(e.UpstreamHeaders),
	}
}

func validUTF8Map(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = strings.ToValidUTF8(v, "�")
	}
	return out
}

func KoboDeviceProto(d models.KoboDevice) *booksv1.KoboDevice {
	lastSeen := ""
	if d.LastSeenAt != nil {
		lastSeen = d.LastSeenAt.Format(time.RFC3339)
	}
	return &booksv1.KoboDevice{
		Id:             d.ID,
		Name:           d.Name,
		Serial:         d.Serial,
		CreatedAt:      d.CreatedAt.Format(time.RFC3339),
		LastSeenAt:     lastSeen,
		LoggingEnabled: d.LoggingEnabled,
	}
}

func KoboStoreBookProto(b models.KoboStoreBookStatus) *booksv1.KoboStoreBook {
	out := &booksv1.KoboStoreBook{
		EntitlementId:  b.EntitlementID,
		Title:          b.Title,
		Authors:        b.Authors,
		Owned:          b.Owned != nil && *b.Owned,
		LastOutcome:    string(b.LastOutcome),
		LastReadAt:     formatOptionalTime(b.LastReadAt),
		LastMirroredAt: formatOptionalTime(b.LastMirroredAt),
		UpdatedAt:      b.UpdatedAt.Format(time.RFC3339),
	}
	if b.ISBN13 != nil {
		out.Isbn13 = *b.ISBN13
	}
	if b.LastPercent != nil {
		p := int32(*b.LastPercent) //nolint:gosec // a 0-100 percent
		out.LastPercent = &p
	}
	if b.Match != nil {
		out.LibraryBookId = b.Match.BookID.String()
		if b.Match.Book != nil {
			out.LibraryTitle = b.Match.Book.Title
		}
	}
	return out
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func ReadingPositionToProto(p models.ReadingPosition) *booksv1.ReadingPosition {
	return &booksv1.ReadingPosition{
		Href:   p.Href,
		Offset: Int32FromInt(p.Offset),
		Page:   Int32FromInt(p.Page),
	}
}

func ReadingStateToProto(state *models.BookReadingState) *booksv1.BookReadingStateData {
	out := &booksv1.BookReadingStateData{
		Source:    state.Source,
		Percent:   Int32FromInt(state.Percent),
		Location:  StringPtr(state.Location),
		UpdatedAt: state.UpdatedAt.Format(time.RFC3339),
	}
	if state.Position != nil {
		out.Position = ReadingPositionToProto(*state.Position)
	}
	if state.ReadAt != nil {
		out.ReadAt = state.ReadAt.UTC().Format(time.RFC3339)
	}
	return out
}
