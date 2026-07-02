package backlog

import (
	"fmt"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
)

// coverProxyURL returns the proxy URL for a book cover served through our own
// endpoint. When coverBaseURL is empty (e.g. in tests) it returns an empty
// string, and the book will have no cover_url in the proto response.
func coverProxyURL(bookID fmt.Stringer, coverBaseURL string) string {
	if coverBaseURL == "" {
		return ""
	}

	return coverBaseURL + "/backlog/api/cover/" + bookID.String()
}

func protoBook(book *models.Book, coverBaseURL string) *backlogv1.Book {
	if book == nil {
		return nil
	}

	// Only expose a cover URL when the book has one stored. The actual image is
	// served through our proxy endpoint (which caches it in R2) rather than
	// directly from Open Library.
	proxyURL := ""
	if book.CoverURL != nil && *book.CoverURL != "" {
		proxyURL = coverProxyURL(book.ID, coverBaseURL)
	}

	return &backlogv1.Book{
		Id:          book.ID.String(),
		Title:       book.Title,
		Authors:     book.Authors,
		Isbn13:      stringPtr(book.ISBN13),
		CoverUrl:    proxyURL,
		Description: stringPtr(book.Description),
		PageCount:   int32FromIntPtr(book.PageCount),
	}
}

func protoUserBook(ub models.UserBook, coverBaseURL string) *backlogv1.UserBook {
	finishedAt := make([]string, len(ub.FinishedAt))
	for i, t := range ub.FinishedAt {
		finishedAt[i] = t.Format(time.RFC3339)
	}

	return &backlogv1.UserBook{
		Id:              ub.ID.String(),
		UserId:          ub.UserID,
		BookId:          ub.BookID.String(),
		Book:            protoBook(ub.Book, coverBaseURL),
		Status:          ub.Status,
		Tags:            ub.Tags,
		Formats:         ub.Formats,
		Rating:          int32PtrFromInt16(ub.Rating),
		FinishedAt:      finishedAt,
		ProgressMode:    ub.ProgressMode,
		CurrentPage:     int32FromInt(ub.CurrentPage),
		ProgressPercent: int32FromInt(ub.ProgressPercent),
		AddedAt:         ub.AddedAt.Format(time.RFC3339),
		UpdatedAt:       ub.UpdatedAt.Format(time.RFC3339),
	}
}

func protoUserBooks(
	books []models.UserBook,
	coverBaseURL string,
) []*backlogv1.UserBook {
	result := make([]*backlogv1.UserBook, len(books))
	for i, b := range books {
		result[i] = protoUserBook(b, coverBaseURL)
	}

	return result
}

func protoBookshelves(shelves []bookShelf, coverBaseURL string) []*backlogv1.BookShelf {
	result := make([]*backlogv1.BookShelf, len(shelves))
	for i, s := range shelves {
		result[i] = &backlogv1.BookShelf{
			Name:  s.Name,
			Books: protoUserBooks(s.Books, coverBaseURL),
		}
	}
	return result
}

func protoExternalBooks(
	books []openlibrary.ExternalBook,
) []*backlogv1.ExternalBookResult {
	result := make([]*backlogv1.ExternalBookResult, len(books))
	for i, b := range books {
		result[i] = &backlogv1.ExternalBookResult{
			Provider:    b.Provider,
			ProviderId:  b.ProviderID,
			Title:       b.Title,
			Authors:     b.Authors,
			Isbn13:      stringPtr(b.ISBN13),
			CoverUrl:    stringPtr(b.CoverURL),
			Description: stringPtr(b.Description),
		}
	}
	return result
}
