package services

import (
	"context"
	"math"
	"sort"
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
) ([]int, [][]models.Game, error) {
	games, err := s.steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// 11 buckets: [0-9], [10-19], ..., [90-99], [100]
	const buckets = 11
	const maxCompletionRate = 100.0
	counts := make([]int, buckets)
	bucketGames := make([][]models.Game, buckets)
	for i := range bucketGames {
		bucketGames[i] = []models.Game{}
	}

	for _, game := range games {
		rate, parseErr := strconv.ParseFloat(game.CompletionRate, 64)
		if parseErr != nil || rate <= 0 {
			continue
		}

		var bucket int
		if rate >= maxCompletionRate {
			bucket = buckets - 1 // last bucket
		} else {
			bucket = int(math.Floor(rate / (buckets - 1)))
		}
		counts[bucket]++
		bucketGames[bucket] = append(bucketGames[bucket], game)
	}

	for i := range bucketGames {
		sort.Slice(bucketGames[i], func(a, b int) bool {
			rateA, _ := strconv.ParseFloat(bucketGames[i][a].CompletionRate, 64)
			rateB, _ := strconv.ParseFloat(bucketGames[i][b].CompletionRate, 64)
			if rateA != rateB {
				return rateA < rateB
			}
			return bucketGames[i][a].Name < bucketGames[i][b].Name
		})
	}

	return counts, bucketGames, nil
}
