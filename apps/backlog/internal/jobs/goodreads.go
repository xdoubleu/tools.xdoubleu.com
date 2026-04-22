package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
	"tools.xdoubleu.com/internal/auth"
)

type GoodreadsJob struct {
	authService      auth.Service
	goodreadsService *services.GoodreadsService
	progressService  *services.ProgressService
}

func NewGoodreadsJob(
	authService auth.Service,
	goodreadsService *services.GoodreadsService,
	progressService *services.ProgressService,
) GoodreadsJob {
	return GoodreadsJob{
		authService:      authService,
		goodreadsService: goodreadsService,
		progressService:  progressService,
	}
}

func (j GoodreadsJob) ID() string {
	return "goodreads"
}

func (j GoodreadsJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j GoodreadsJob) Run(ctx context.Context, logger *slog.Logger) error {
	users, err := j.authService.GetAllUsers()
	if err != nil {
		return err
	}

	var errs []error
	for _, user := range users {
		if err = j.runForUser(ctx, logger, user.ID); err != nil {
			logger.ErrorContext(ctx, "goodreads job failed for user",
				slog.String("userID", user.ID),
				slog.Any("error", err),
			)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (j GoodreadsJob) runForUser(
	ctx context.Context,
	logger *slog.Logger,
	userID string,
) error {
	logger.DebugContext(ctx, "fetching books")
	books, err := j.goodreadsService.ImportAllBooks(ctx, userID)
	if err != nil {
		return err
	}
	if books == nil {
		logger.DebugContext(ctx, "goodreads not configured for user", "userID", userID)
		return nil
	}
	logger.DebugContext(ctx, fmt.Sprintf("fetched %d books", len(books)))

	labels, values := buildReadProgress(books)
	logger.DebugContext(ctx, fmt.Sprintf("read %d books total", len(labels)))

	logger.DebugContext(ctx, "saving progress")
	return j.progressService.Save(ctx, models.GoodreadsTypeID, userID, labels, values)
}

func buildReadProgress(books []goodreads.Book) ([]string, []string) {
	uniqueDates := []string{}
	for _, book := range books {
		for _, d := range book.DatesRead {
			dateStr := d.Format(models.ProgressDateFormat)
			if !slices.Contains(uniqueDates, dateStr) {
				uniqueDates = append(uniqueDates, dateStr)
			}
		}
	}
	slices.Sort(uniqueDates)

	labels := make([]string, 0, len(uniqueDates))
	values := make([]string, 0, len(uniqueDates))
	cumulative := 0
	for _, dateStr := range uniqueDates {
		count := countBooksReadOn(books, dateStr)
		cumulative += count
		labels = append(labels, dateStr)
		values = append(values, fmt.Sprintf("%d", cumulative))
	}

	return labels, values
}

func countBooksReadOn(books []goodreads.Book, dateStr string) int {
	count := 0
	for _, book := range books {
		for _, d := range book.DatesRead {
			if d.Format(models.ProgressDateFormat) == dateStr {
				count++
			}
		}
	}
	return count
}
