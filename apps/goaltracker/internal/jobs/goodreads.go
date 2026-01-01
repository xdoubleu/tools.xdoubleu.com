package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/XDoubleU/essentia/pkg/grapher"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/apps/goaltracker/internal/services"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/internal/auth"
)

type GoodreadsJob struct {
	authService      auth.Service
	goodreadsService *services.GoodreadsService
	goalService      *services.GoalService
}

func NewGoodreadsJob(
	authService auth.Service,
	goodreadsService *services.GoodreadsService,
	goalService *services.GoalService,
) GoodreadsJob {
	return GoodreadsJob{
		authService:      authService,
		goodreadsService: goodreadsService,
		goalService:      goalService,
	}
}

func (j GoodreadsJob) ID() string {
	return strconv.Itoa(int(models.GoodreadsSource.ID))
}

func (j GoodreadsJob) RunEvery() time.Duration {
	//nolint:mnd //no magic number
	return 24 * time.Hour
}

func (j GoodreadsJob) Run(ctx context.Context, logger *slog.Logger) error {
	users, err := j.authService.GetAllUsers()
	if err != nil {
		return err
	}

	for _, user := range users {
		logger.Debug("fetching books")
		var books []goodreads.Book
		books, err = j.goodreadsService.ImportAllBooks(ctx, user.ID)
		if err != nil {
			return err
		}
		logger.Debug(fmt.Sprintf("fetched %d books", len(books)))

		err = j.updateProgress(ctx, logger, user.ID, books)
		if err != nil {
			return err
		}
	}

	return nil
}

func (j GoodreadsJob) updateProgress(
	ctx context.Context,
	logger *slog.Logger,
	userID string,
	books []goodreads.Book,
) error {
	graphers := map[int]*grapher.Grapher[int]{}

	graphers[time.Now().Year()] = grapher.New[int](
		grapher.Cumulative,
		grapher.PreviousValue,
		models.ProgressDateFormat,
		24*time.Hour, //nolint:mnd //no magic number
	)
	graphers[time.Now().Year()].AddPoint(
		time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC),
		0,
		"",
	)
	graphers[time.Now().Year()].AddPoint(
		time.Date(
			time.Now().Year(),
			time.Now().Month(),
			time.Now().Day(),
			0,
			0,
			0,
			0,
			time.UTC,
		),
		0,
		"",
	)

	for _, book := range books {
		if len(book.DatesRead) == 0 {
			continue
		}

		for _, dateRead := range book.DatesRead {
			g, ok := graphers[dateRead.Year()]
			if !ok {
				graphers[dateRead.Year()] = grapher.New[int](
					grapher.Cumulative,
					grapher.PreviousValue,
					models.ProgressDateFormat,
					24*time.Hour, //nolint:mnd //no magic number
				)
				g = graphers[dateRead.Year()]
			}

			g.AddPoint(dateRead, 1, "")
		}
	}

	progressLabels, progressValues := []string{}, []string{}
	for _, grapher := range graphers {
		pL, pV := grapher.ToStringSlices()
		progressLabels = append(progressLabels, pL...)
		progressValues = append(progressValues, pV[""]...)
	}

	logger.Debug("saving progress")
	return j.goalService.SaveProgress(
		ctx,
		models.FinishedBooksThisYear.ID,
		userID,
		progressLabels,
		progressValues,
	)
}
