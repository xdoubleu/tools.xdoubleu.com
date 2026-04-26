package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v3/pkg/database"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/books"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

type BookService struct {
	logger          *slog.Logger
	books           *repositories.BooksRepository
	providerFactory func(apiKey string) hardcover.Client
	integrations    *IntegrationsService
}

// SearchLibrary searches the user's own library by title/author substring.
func (s *BookService) SearchLibrary(
	ctx context.Context,
	userID string,
	query string,
) ([]models.UserBook, error) {
	return s.books.SearchLibrary(ctx, userID, query)
}

// SearchHardcover calls the Hardcover API. Returns nil if no API key configured.
func (s *BookService) SearchHardcover(
	ctx context.Context,
	userID string,
	query string,
) ([]hardcover.ExternalBook, error) {
	creds, err := s.integrations.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if creds.HardcoverAPIKey == "" {
		return nil, nil
	}

	return s.providerFactory(creds.HardcoverAPIKey).Search(ctx, query)
}

func (s *BookService) AddToLibrary(
	ctx context.Context,
	userID string,
	ext hardcover.ExternalBook,
	status string,
	initialTags []string,
) (*models.UserBook, error) {
	book := externalToBook(ext)
	saved, err := s.books.UpsertBook(ctx, book)
	if err != nil {
		return nil, err
	}

	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID: userID,
		BookID: saved.ID,
		Status: status,
		Tags:   initialTags,
	}
	if err = s.books.UpsertUserBook(ctx, ub); err != nil {
		return nil, err
	}

	return s.books.GetUserBook(ctx, userID, saved.ID)
}

func (s *BookService) UpdateStatus(
	ctx context.Context,
	_ string,
	ub models.UserBook,
) error {
	return s.books.UpsertUserBook(ctx, ub)
}

// ToggleTag adds or removes a tag from a user_book atomically.
func (s *BookService) ToggleTag(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	tag string,
) error {
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return fmt.Errorf("book not found")
		}
		return err
	}

	newTags := make([]string, 0, len(ub.Tags))
	found := false
	for _, t := range ub.Tags {
		if t == tag {
			found = true
			continue
		}
		newTags = append(newTags, t)
	}
	if !found {
		newTags = append(newTags, tag)
	}

	return s.books.UpdateTags(ctx, userID, bookID, newTags)
}

func (s *BookService) GetUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.UserBook, error) {
	return s.books.GetUserBook(ctx, userID, bookID)
}

func (s *BookService) GetByStatus(
	ctx context.Context,
	userID string,
	status string,
) ([]models.UserBook, error) {
	return s.books.GetByStatus(ctx, userID, status)
}

func (s *BookService) GetLibrary(
	ctx context.Context,
	userID string,
) ([]models.UserBook, error) {
	return s.books.GetLibrary(ctx, userID)
}

// ImportFromCSV parses a Goodreads CSV export and upserts all entries into the library.
// Returns the number of entries successfully imported.
func (s *BookService) ImportFromCSV(
	ctx context.Context,
	userID string,
	r io.Reader,
) (int, error) {
	entries, err := books.ParseCSV(r)
	if err != nil {
		return 0, err
	}

	bookList := make([]models.Book, len(entries))
	ubList := make([]models.UserBook, len(entries))
	for i, e := range entries {
		bookList[i] = e.Book
		ubList[i] = e.UserBook
		ubList[i].UserID = userID
	}

	s.logger.DebugContext(ctx, fmt.Sprintf("importing %d books from CSV", len(entries)))

	if err = s.books.BatchUpsert(ctx, userID, bookList, ubList); err != nil {
		return 0, err
	}

	return len(entries), nil
}

// BuildReadProgress returns sorted cumulative labels+values for the progress chart.
func (s *BookService) BuildReadProgress(
	ctx context.Context,
	userID string,
) ([]string, []string, error) {
	dates, err := s.books.GetFinishedDates(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	uniqueDates := []string{}
	for _, d := range dates {
		ds := d.Format(models.ProgressDateFormat)
		if !slices.Contains(uniqueDates, ds) {
			uniqueDates = append(uniqueDates, ds)
		}
	}
	slices.Sort(uniqueDates)

	labels := make([]string, 0, len(uniqueDates))
	values := make([]string, 0, len(uniqueDates))
	cumulative := 0
	for _, ds := range uniqueDates {
		count := countDatesOn(dates, ds)
		cumulative += count
		labels = append(labels, ds)
		values = append(values, fmt.Sprintf("%d", cumulative))
	}

	return labels, values, nil
}

func countDatesOn(dates []time.Time, dateStr string) int {
	count := 0
	for _, d := range dates {
		if d.Format(models.ProgressDateFormat) == dateStr {
			count++
		}
	}
	return count
}

func externalToBook(ext hardcover.ExternalBook) models.Book {
	return models.Book{ //nolint:exhaustruct //optional fields
		Title:        ext.Title,
		Authors:      ext.Authors,
		ISBN13:       ext.ISBN13,
		ISBN10:       ext.ISBN10,
		CoverURL:     ext.CoverURL,
		Description:  ext.Description,
		ExternalRefs: map[string]string{ext.Provider: ext.ProviderID},
	}
}
