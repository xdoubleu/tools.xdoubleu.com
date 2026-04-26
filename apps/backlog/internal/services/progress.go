package services

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/xdoubleu/essentia/v3/pkg/database"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
)

type ProgressService struct {
	progress *repositories.ProgressRepository
	steam    *SteamService
}

func (s *ProgressService) Save(
	ctx context.Context,
	typeID string,
	userID string,
	dates []string,
	values []string,
) error {
	return s.progress.Upsert(ctx, typeID, userID, dates, values)
}

func (s *ProgressService) GetByTypeIDAndDates(
	ctx context.Context,
	typeID string,
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

	if len(progresses) == 0 {
		return nil, nil, nil
	}

	// Index stored records by date string.
	byDate := make(map[string]string, len(progresses))
	for _, p := range progresses {
		byDate[p.Date.Format(models.ProgressDateFormat)] = p.Value
	}

	// Fill every calendar day from first record to today (or dateEnd).
	const day = 24 * time.Hour
	first := progresses[0].Date.UTC().Truncate(day)
	end := dateEnd.UTC().Truncate(day)
	if today := time.Now().UTC().Truncate(day); today.Before(end) {
		end = today
	}

	labels := make([]string, 0, int(end.Sub(first)/day)+1)
	values := make([]string, 0, len(labels))
	lastValue := ""
	for d := first; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format(models.ProgressDateFormat)
		if v, ok := byDate[ds]; ok {
			lastValue = v
		}
		labels = append(labels, ds)
		values = append(values, lastValue)
	}

	return labels, values, nil
}

func (s *ProgressService) GetCurrentSteamCompletionRate(
	ctx context.Context,
	userID string,
) (string, error) {
	value, err := s.progress.GetLatestByTypeID(ctx, models.SteamTypeID, userID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return "0.00", nil
		}
		return "", err
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
