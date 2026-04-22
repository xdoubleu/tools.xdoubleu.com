package services

import (
	"context"
	"math"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
)

type ProgressService struct {
	progress *repositories.ProgressRepository
	steam    *SteamService
}

func (s *ProgressService) Save(
	ctx context.Context,
	typeID int64,
	userID string,
	dates []string,
	values []string,
) error {
	return s.progress.Upsert(ctx, typeID, userID, dates, values)
}

func (s *ProgressService) GetByTypeIDAndDates(
	ctx context.Context,
	typeID int64,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]string, []string, error) {
	progresses, err := s.progress.GetByTypeIDAndDates(
		ctx, typeID, userID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, nil, err
	}

	labels := []string{}
	values := []string{}
	for _, p := range progresses {
		labels = append(labels, p.Date.Format(models.ProgressDateFormat))
		values = append(values, p.Value)
	}

	return labels, values, nil
}

func (s *ProgressService) GetCurrentSteamCompletionRate(
	ctx context.Context,
	userID string,
) (string, error) {
	value, err := s.progress.GetLatestByTypeID(ctx, models.SteamTypeID, userID)
	if err != nil {
		// no progress recorded yet
		return "0.00", nil //nolint:nilerr // absence of data is not an error
	}
	return value, nil
}

func (s *ProgressService) GetCompletionRateDistribution(
	ctx context.Context,
	userID string,
) ([]int, error) {
	games, err := s.steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, err
	}

	//nolint:mnd // 11 buckets: [0-9], [10-19], ..., [90-99], [100]
	counts := make([]int, 11)
	for _, game := range games {
		rate, parseErr := strconv.ParseFloat(game.CompletionRate, 64)
		if parseErr != nil || rate <= 0 {
			continue
		}

		var bucket int
		if rate >= 100 { //nolint:mnd // 100% is its own bucket
			bucket = 10
		} else {
			//nolint:mnd // floor(rate/10) gives bucket index 0-9
			bucket = int(math.Floor(rate / 10))
		}
		counts[bucket]++
	}

	return counts, nil
}
