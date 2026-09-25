// Package progresshistory stores cumulative progress per (user, date); reads
// fill every day in the window, carrying the last earlier value forward.
package progresshistory

import (
	"context"
	"errors"
	"time"

	"tools.xdoubleu.com/internal/database"
)

// DateFormat is the wire format for progress dates.
const DateFormat = "2006-01-02"

// Record is a single stored progress value.
type Record struct {
	Date  time.Time
	Value string
}

// Repository is implemented in each app's repositories package.
type Repository interface {
	Upsert(
		ctx context.Context,
		userID string,
		dates []string,
		values []string,
	) error
	GetByDates(
		ctx context.Context,
		userID string,
		dateStart time.Time,
		dateEnd time.Time,
	) ([]Record, error)
	GetLastValueBefore(
		ctx context.Context,
		userID string,
		date time.Time,
	) (string, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(
	ctx context.Context,
	userID string,
	dates []string,
	values []string,
) error {
	return s.repo.Upsert(ctx, userID, dates, values)
}

// GetByDates returns per-day labels and values, carrying values forward.
func (s *Service) GetByDates(
	ctx context.Context,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]string, []string, error) {
	baseline, err := s.repo.GetLastValueBefore(ctx, userID, dateStart)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return nil, nil, err
	}

	progresses, err := s.repo.GetByDates(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, nil, err
	}

	if baseline == "" && len(progresses) == 0 {
		return nil, nil, nil
	}

	byDate := make(map[string]string, len(progresses))
	for _, p := range progresses {
		byDate[p.Date.Format(DateFormat)] = p.Value
	}

	const day = 24 * time.Hour
	start := dateStart.UTC().Truncate(day)
	end := dateEnd.UTC().Truncate(day)
	if today := time.Now().UTC().Truncate(day); today.Before(end) {
		end = today
	}

	labels := make([]string, 0, int(end.Sub(start)/day)+1)
	values := make([]string, 0, len(labels))
	lastValue := baseline
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format(DateFormat)
		if v, ok := byDate[ds]; ok {
			lastValue = v
		}
		labels = append(labels, ds)
		values = append(values, lastValue)
	}

	return labels, values, nil
}
