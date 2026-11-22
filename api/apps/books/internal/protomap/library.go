package protomap

import (
	"fmt"
	"slices"
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// CoverProxyURL returns the proxy URL for a book cover served through our own
// endpoint. When coverBaseURL is empty (e.g. in tests) it returns an empty
// string, and the book will have no cover_url in the proto response.
func CoverProxyURL(bookID fmt.Stringer, coverBaseURL string) string {
	if coverBaseURL == "" {
		return ""
	}

	return coverBaseURL + "/books/api/cover/" + bookID.String()
}

func ProtoBook(book *models.Book, coverBaseURL string) *booksv1.Book {
	if book == nil {
		return nil
	}

	// Only expose a cover URL when the book has one stored. The actual image is
	// served through our proxy endpoint from R2 (fetched there eagerly at
	// write time — see BookService.cacheCoverFromURL), not from the source
	// directly.
	proxyURL := ""
	if book.CoverURL != nil && *book.CoverURL != "" {
		proxyURL = CoverProxyURL(book.ID, coverBaseURL)
	}

	out := &booksv1.Book{
		Id:          book.ID.String(),
		Title:       book.Title,
		Authors:     book.Authors,
		Isbn13:      StringPtr(book.ISBN13),
		CoverUrl:    proxyURL,
		Description: StringPtr(book.Description),
		PageCount:   Int32FromIntPtr(book.PageCount),
		SourceUrl:   StringPtr(book.SourceURL),
		HasContent:  book.HasContent,
	}
	if book.Series != nil {
		out.SeriesName = book.Series.Name
		out.SeriesPosition = book.Series.Position
		out.SeriesTotal = Int32FromIntPtr(book.Series.Total)
	}
	return out
}

// reconcileOwnDigitalTag derives the own-digital tag from actual attached
// formats rather than trusting the stored tag, since it can drift out of
// sync (CSV import setting it verbatim, or a stale tag left over from a
// removed file/merge).
func reconcileOwnDigitalTag(tags, formats []string) []string {
	hasDigital := len(formats) > 0
	tagged := slices.Contains(tags, models.TagOwnDigital)
	if hasDigital == tagged {
		return tags
	}

	if hasDigital {
		return append(slices.Clone(tags), models.TagOwnDigital)
	}

	return slices.DeleteFunc(slices.Clone(tags), func(t string) bool {
		return t == models.TagOwnDigital
	})
}

// ProtoUserBook maps one library entry onto the wire type.
func ProtoUserBook(ub models.UserBook, coverBaseURL string) *booksv1.UserBook {
	finishedAt := make([]string, len(ub.FinishedAt))
	for i, t := range ub.FinishedAt {
		finishedAt[i] = t.Format(time.RFC3339)
	}

	return &booksv1.UserBook{
		Id:                ub.ID.String(),
		UserId:            ub.UserID,
		BookId:            ub.BookID.String(),
		Book:              ProtoBook(ub.Book, coverBaseURL),
		Status:            ub.Status,
		Tags:              reconcileOwnDigitalTag(ub.Tags, ub.Formats),
		Formats:           ub.Formats,
		FileVersions:      ub.FileVersions,
		ReadSuggestion:    protoReadSuggestion(ub.ReadSuggestion),
		Rating:            Int32PtrFromInt16(ub.Rating),
		FinishedAt:        finishedAt,
		ProgressMode:      ub.ProgressMode,
		CurrentPage:       Int32FromInt(ub.CurrentPage),
		ProgressPercent:   Int32FromInt(ub.ProgressPercent),
		PhysicalPageCount: Int32FromIntPtr(ub.PhysicalPageCount),
		AddedAt:           ub.AddedAt.Format(time.RFC3339),
		UpdatedAt:         ub.UpdatedAt.Format(time.RFC3339),
	}
}

func protoReadSuggestion(s *models.ReadSuggestion) *booksv1.ReadSuggestion {
	if s == nil {
		return nil
	}
	return &booksv1.ReadSuggestion{
		Format:      s.Format,
		FigureCount: Int32FromInt(s.FigureCount),
		Pages:       Int32FromInt(s.Pages),
	}
}

// ProtoUserBooks maps library entries onto the wire type.
func ProtoUserBooks(
	books []models.UserBook,
	coverBaseURL string,
) []*booksv1.UserBook {
	result := make([]*booksv1.UserBook, len(books))
	for i, b := range books {
		result[i] = ProtoUserBook(b, coverBaseURL)
	}

	return result
}

// ProtoShelves maps shelves onto the wire type.
func ProtoShelves(shelves []services.Shelf, coverBaseURL string) []*booksv1.BookShelf {
	result := make([]*booksv1.BookShelf, len(shelves))
	for i, s := range shelves {
		result[i] = &booksv1.BookShelf{
			Name:  s.Name,
			Books: ProtoUserBooks(s.Books, coverBaseURL),
		}
	}
	return result
}

// ProtoExternalBook maps a search/detail result onto the wire type.
// ProviderId is the ISBN13 by convention — both Hardcover and UniCat only
// support fetch-by-ISBN, so that's what GetExternalBook routes on.
func ProtoExternalBook(b services.SourceProposal) *booksv1.ExternalBookResult {
	return &booksv1.ExternalBookResult{
		Provider:    b.Source,
		ProviderId:  b.ISBN13,
		Title:       b.Title,
		Authors:     b.Authors,
		Isbn13:      b.ISBN13,
		CoverUrl:    b.CoverURL,
		Description: b.Description,

		SeriesName:     b.SeriesName,
		SeriesPosition: b.SeriesPosition,
		SeriesTotal:    Int32FromIntPtr(b.SeriesTotal),
	}
}

func ProtoExternalBooks(
	books []services.SourceProposal,
) []*booksv1.ExternalBookResult {
	result := make([]*booksv1.ExternalBookResult, len(books))
	for i, b := range books {
		result[i] = ProtoExternalBook(b)
	}
	return result
}
