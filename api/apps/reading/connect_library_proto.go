package reading

import (
	"fmt"
	"time"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// coverProxyURL returns the proxy URL for a book cover served through our own
// endpoint. When coverBaseURL is empty (e.g. in tests) it returns an empty
// string, and the book will have no cover_url in the proto response.
func coverProxyURL(bookID fmt.Stringer, coverBaseURL string) string {
	if coverBaseURL == "" {
		return ""
	}

	return coverBaseURL + "/reading/api/cover/" + bookID.String()
}

func protoBook(book *models.Book, coverBaseURL string) *readingv1.Book {
	if book == nil {
		return nil
	}

	// Only expose a cover URL when the book has one stored. The actual image is
	// served through our proxy endpoint from R2 (fetched there eagerly at
	// write time — see BookService.cacheCoverFromURL), not from the source
	// directly.
	proxyURL := ""
	if book.CoverURL != nil && *book.CoverURL != "" {
		proxyURL = coverProxyURL(book.ID, coverBaseURL)
	}

	category := book.Category
	if category == "" {
		category = models.CategoryBook
	}

	return &readingv1.Book{
		Id:          book.ID.String(),
		Title:       book.Title,
		Authors:     book.Authors,
		Isbn13:      stringPtr(book.ISBN13),
		CoverUrl:    proxyURL,
		Description: stringPtr(book.Description),
		PageCount:   int32FromIntPtr(book.PageCount),
		Category:    category,
		SourceUrl:   stringPtr(book.SourceURL),
	}
}

func protoUserBook(ub models.UserBook, coverBaseURL string) *readingv1.UserBook {
	finishedAt := make([]string, len(ub.FinishedAt))
	for i, t := range ub.FinishedAt {
		finishedAt[i] = t.Format(time.RFC3339)
	}

	return &readingv1.UserBook{
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
) []*readingv1.UserBook {
	result := make([]*readingv1.UserBook, len(books))
	for i, b := range books {
		result[i] = protoUserBook(b, coverBaseURL)
	}

	return result
}

func protoBookshelves(shelves []bookShelf, coverBaseURL string) []*readingv1.BookShelf {
	result := make([]*readingv1.BookShelf, len(shelves))
	for i, s := range shelves {
		result[i] = &readingv1.BookShelf{
			Name:  s.Name,
			Books: protoUserBooks(s.Books, coverBaseURL),
		}
	}
	return result
}

// protoExternalBook maps a search/detail result onto the wire type.
// ProviderId is the ISBN13 by convention — both Hardcover and UniCat only
// support fetch-by-ISBN, so that's what GetExternalBook routes on.
func protoExternalBook(b services.SourceProposal) *readingv1.ExternalBookResult {
	return &readingv1.ExternalBookResult{
		Provider:    b.Source,
		ProviderId:  b.ISBN13,
		Title:       b.Title,
		Authors:     b.Authors,
		Isbn13:      b.ISBN13,
		CoverUrl:    b.CoverURL,
		Description: b.Description,
	}
}

func protoExternalBooks(
	books []services.SourceProposal,
) []*readingv1.ExternalBookResult {
	result := make([]*readingv1.ExternalBookResult, len(books))
	for i, b := range books {
		result[i] = protoExternalBook(b)
	}
	return result
}
