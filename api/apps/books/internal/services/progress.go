package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/internal/progresshistory"
)

// progressRepoAdapter adapts ProgressRepository to the storage interface of
// the shared progresshistory service.
type progressRepoAdapter struct {
	repo *repositories.ProgressRepository
}

func (a progressRepoAdapter) Upsert(
	ctx context.Context,
	typeID string,
	userID string,
	dates []string,
	values []string,
) error {
	return a.repo.Upsert(ctx, nil, typeID, userID, dates, values)
}

func (a progressRepoAdapter) GetByTypeIDAndDates(
	ctx context.Context,
	typeID string,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]progresshistory.Record, error) {
	progresses, err := a.repo.GetByTypeIDAndDates(
		ctx, typeID, userID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, err
	}
	records := make([]progresshistory.Record, len(progresses))
	for i, p := range progresses {
		records[i] = progresshistory.Record{
			TypeID: p.TypeID,
			Date:   p.Date,
			Value:  p.Value,
		}
	}
	return records, nil
}

func (a progressRepoAdapter) GetLastValueBefore(
	ctx context.Context,
	typeID string,
	userID string,
	date time.Time,
) (string, error) {
	return a.repo.GetLastValueBefore(ctx, typeID, userID, date)
}

// ProgressService stores and reads the cumulative books-read progress graph.
type ProgressService struct {
	history *progresshistory.Service
}

func NewProgressService(progress *repositories.ProgressRepository) *ProgressService {
	return &ProgressService{
		history: progresshistory.NewService(progressRepoAdapter{repo: progress}),
	}
}

func (s *ProgressService) Save(
	ctx context.Context,
	typeID string,
	userID string,
	dates []string,
	values []string,
) error {
	return s.history.Save(ctx, typeID, userID, dates, values)
}

func (s *ProgressService) GetByTypeIDAndDates(
	ctx context.Context,
	typeID string,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]string, []string, error) {
	return s.history.GetByTypeIDAndDates(ctx, typeID, userID, dateStart, dateEnd)
}
