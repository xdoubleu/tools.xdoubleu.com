package services

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/internal/progresshistory"
)

// progressRepoAdapter adapts ProgressRepository to the storage interface of
// the shared progresshistory service (non-transactional writes).
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
		history:  progresshistory.NewService(progressRepoAdapter{repo: progress}),
		progress: progress,
		steam:    steam,
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
