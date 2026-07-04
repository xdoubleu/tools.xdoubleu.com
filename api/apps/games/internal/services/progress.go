package services

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/games/internal/repositories"
	"tools.xdoubleu.com/internal/progresshistory"
)

type ProgressService struct {
	history  *progresshistory.Service
	progress *repositories.ProgressRepository
	steam    *SteamService
}

func NewProgressService(
	progress *repositories.ProgressRepository,
	steam *SteamService,
) *ProgressService {
	return &ProgressService{
		history:  progresshistory.NewService(progress),
		progress: progress,
		steam:    steam,
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

func (s *ProgressService) GetCurrentSteamCompletionRate(
	ctx context.Context,
	userID string,
) (string, error) {
	value, err := s.progress.GetLatest(ctx, userID)
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
