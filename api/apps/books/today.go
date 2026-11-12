package books

import (
	"context"
	"slices"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// ReadingBook is a book being read, for the Today page.
type ReadingBook struct {
	ID              string
	Title           string
	Authors         []string
	ProgressPercent int
}

// ListCurrentlyReading returns up to limit of userID's books being read, most
// recently updated first. URL-ingested articles aren't books and are left out.
func (app *Books) ListCurrentlyReading(
	ctx context.Context,
	userID string,
	limit int,
) ([]ReadingBook, error) {
	library, err := app.services.Books.GetLibrary(ctx, userID)
	if err != nil {
		return nil, err
	}

	reading := make([]models.UserBook, 0, limit)
	for _, ub := range library {
		if ub.Status == models.StatusReading && ub.Book != nil && ub.Book.SourceURL == nil {
			reading = append(reading, ub)
		}
	}
	slices.SortStableFunc(reading, func(a, b models.UserBook) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})

	books := make([]ReadingBook, 0, min(limit, len(reading)))
	for _, ub := range reading[:min(limit, len(reading))] {
		books = append(books, ReadingBook{
			ID:              ub.BookID.String(),
			Title:           ub.Book.Title,
			Authors:         ub.Book.Authors,
			ProgressPercent: ub.DisplayProgressPercent(),
		})
	}
	return books, nil
}
