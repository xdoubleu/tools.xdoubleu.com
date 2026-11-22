package protomap

import (
	"math"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func protoSeries(
	view *services.SeriesView,
	coverBaseURL string,
) *booksv1.GetSeriesResponse {
	entries := make([]*booksv1.SeriesEntry, len(view.Entries))
	for i, e := range view.Entries {
		entry := &booksv1.SeriesEntry{
			Position: e.Position,
		}
		if e.UserBook != nil {
			entry.UserBook = ProtoUserBook(*e.UserBook, coverBaseURL)
		}
		if e.External != nil {
			entry.External = ProtoExternalBook(*e.External)
		}
		entries[i] = entry
	}
	return &booksv1.GetSeriesResponse{
		Name:                view.Name,
		Total:               Int32FromIntPtr(view.Total),
		Entries:             entries,
		ExternalUnavailable: view.ExternalUnavailable,
	}
}

// SeriesFromProto trims the name (empty means no series) and drops a
// position that isn't a finite non-negative number.
func SeriesFromProto(name string, position *float64) *models.BookSeries {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if position != nil &&
		(*position < 0 || math.IsNaN(*position) || math.IsInf(*position, 0)) {
		position = nil
	}
	return &models.BookSeries{Name: name, Position: position, Total: nil}
}
