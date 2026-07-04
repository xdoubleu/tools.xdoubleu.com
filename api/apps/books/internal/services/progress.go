package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/internal/progresshistory"
)

// ProgressService stores and reads the cumulative books-read progress graph.
type ProgressService struct {
	history *progresshistory.Service
}

func NewProgressService(progress *repositories.ProgressRepository) *ProgressService {
	return &ProgressService{
		history: progresshistory.NewService(progress),
	}
}

func (s *ProgressService) Save(
	ctx context.Context,
	userID string,
	dates []string,
	values []string,
) error {
	return s.history.Save(ctx, userID, dates, values)
}

func (s *ProgressService) GetByDates(
	ctx context.Context,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]string, []string, error) {
	return s.history.GetByDates(ctx, userID, dateStart, dateEnd)
}
