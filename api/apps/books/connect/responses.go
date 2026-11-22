package connect

import (
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/kobo"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// The response builders below turn service results into wire messages. They
// hold no rules, so the Connect handlers and the MCP tools share them.

// LibraryResponse is the wire form of a grouped library.
func LibraryResponse(data services.LibraryView, base string) *booksv1.LibraryResponse {
	return &booksv1.LibraryResponse{
		Reading:  ProtoUserBooks(data.Reading, base),
		Wishlist: ProtoUserBooks(data.Wishlist, base),
		Finished: ProtoUserBooks(data.Finished, base),
		Shelves:  ProtoShelves(data.Shelves, base),
	}
}

// ProgressResponse is the wire form of a progress graph over a window.
func ProgressResponse(
	labels, values []string,
	start, end time.Time,
) *booksv1.BooksProgressResponse {
	return &booksv1.BooksProgressResponse{
		Labels:    labels,
		Values:    values,
		DateStart: start.Format(models.ProgressDateFormat),
		DateEnd:   end.Format(models.ProgressDateFormat),
	}
}

// ExternalBooksResponse is the wire form of external search results.
func ExternalBooksResponse(
	results []services.SourceProposal,
) *booksv1.SearchExternalResponse {
	return &booksv1.SearchExternalResponse{Results: protoExternalBooks(results)}
}

// ExternalBookResponse is the wire form of one external book.
func ExternalBookResponse(b services.SourceProposal) *booksv1.GetExternalBookResponse {
	return &booksv1.GetExternalBookResponse{Result: protoExternalBook(b)}
}

// SeriesResponse is the wire form of a series view.
func SeriesResponse(view *services.SeriesView, base string) *booksv1.GetSeriesResponse {
	return protoSeries(view, base)
}

// ReadingStateResponse is the wire form of a reading state view; a nil view is
// the empty state.
func ReadingStateResponse(
	view *services.ReadingStateView,
) *booksv1.GetReadingStateResponse {
	if view == nil {
		return &booksv1.GetReadingStateResponse{}
	}
	out := readingStateToProto(view.State)
	if view.HasPosition {
		out.PhysicalPage = Int32FromInt(view.PhysicalPage)
		out.PhysicalExcerpt = view.PhysicalExcerpt
		out.Position = readingPositionToProto(view.Position)
	}
	return &booksv1.GetReadingStateResponse{State: out}
}

// BookFileResponse is the wire form of a book file download.
func BookFileResponse(r *services.GetBookFileResult) *booksv1.GetBookFileResponse {
	return &booksv1.GetBookFileResponse{
		Url:       r.URL,
		ExpiresAt: r.ExpiresAt.Format(time.RFC3339),
		Format:    r.Format,
	}
}

// KEPUBStatusResponse is the wire form of a book's KEPUB status.
func KEPUBStatusResponse(v services.KEPUBStatusView) *booksv1.GetKEPUBStatusResponse {
	return &booksv1.GetKEPUBStatusResponse{
		HasEpub:     v.HasEPUB,
		HasPdf:      v.HasPDF,
		KepubStatus: v.KepubStatus,
	}
}

// KoboDevicesResponse is the wire form of a device list.
func KoboDevicesResponse(devices []models.KoboDevice) *booksv1.ListKoboDevicesResponse {
	resp := &booksv1.ListKoboDevicesResponse{
		Devices: make([]*booksv1.KoboDevice, len(devices)),
	}
	for i, d := range devices {
		resp.Devices[i] = koboDeviceProto(d)
	}
	return resp
}

// KoboStoreBooksResponse is the wire form of the Kobo store books list.
func KoboStoreBooksResponse(
	books []models.KoboStoreBookStatus,
) *booksv1.ListKoboStoreBooksResponse {
	resp := &booksv1.ListKoboStoreBooksResponse{
		Books: make([]*booksv1.KoboStoreBook, len(books)),
	}
	for i, b := range books {
		resp.Books[i] = koboStoreBookProto(b)
	}
	return resp
}

// KoboLogsResponse is the wire form of a device's captured logs.
func KoboLogsResponse(entries []kobo.LogEntry) *booksv1.GetKoboDeviceLogsResponse {
	resp := &booksv1.GetKoboDeviceLogsResponse{
		Entries: make([]*booksv1.KoboLogEntry, len(entries)),
	}
	for i, e := range entries {
		resp.Entries[i] = koboLogEntryProto(e)
	}
	return resp
}
