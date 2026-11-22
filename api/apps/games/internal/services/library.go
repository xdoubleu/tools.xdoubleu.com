package services

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/internal/kit/database"
)

// bouncedMinutes is the playtime under which a game counts as bounced off.
const bouncedMinutes = 60

// fullCompletion is models.Game.CompletionRate at every achievement.
const fullCompletion = 100.0

// LibraryService composes the Steam and progress services into the payloads
// the app serves and the taste app consumes.
type LibraryService struct {
	steam    *SteamService
	progress *ProgressService
}

func NewLibraryService(steam *SteamService, progress *ProgressService) *LibraryService {
	return &LibraryService{steam: steam, progress: progress}
}

// GetSteam assembles userID's library payload over the date window.
func (s *LibraryService) GetSteam(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*models.SteamOverview, error) {
	notStarted, err := s.steam.GetBacklog(ctx, userID)
	if err != nil {
		return nil, err
	}

	inProgress, err := s.steam.GetInProgress(ctx, userID)
	if err != nil {
		return nil, err
	}

	distribution, _, err := s.progress.GetCompletionRateDistribution(ctx, userID)
	if err != nil {
		return nil, err
	}

	completed, err := s.steam.GetCompleted(ctx, userID)
	if err != nil {
		return nil, err
	}

	delisted, err := s.steam.GetDelisted(ctx, userID)
	if err != nil {
		return nil, err
	}

	currentRate, err := s.progress.GetCurrentSteamCompletionRate(ctx, userID)
	if err != nil {
		return nil, err
	}

	labels, values, err := s.progress.GetByDates(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, err
	}

	return &models.SteamOverview{
		NotStarted:   notStarted,
		InProgress:   inProgress,
		Completed:    completed,
		Delisted:     delisted,
		TotalBacklog: len(notStarted) + len(inProgress),
		Distribution: distribution,
		CurrentRate:  currentRate,
		Labels:       labels,
		Values:       values,
		DateStart:    dateStart,
		DateEnd:      dateEnd,
	}, nil
}

// GetSteamGame returns one game with achievements sorted by global percent.
func (s *LibraryService) GetSteamGame(
	ctx context.Context,
	userID string,
	gameID int,
) (*models.SteamGameDetail, error) {
	game, err := s.steam.GetGameByID(ctx, gameID, userID)
	if err != nil {
		return nil, err
	}

	achievements, err := s.steam.GetAchievementsForGame(ctx, gameID, userID)
	if err != nil {
		return nil, err
	}

	sortAchievements(achievements)

	return &models.SteamGameDetail{Game: *game, Achievements: achievements}, nil
}

// sortAchievements orders by global percent descending; achievements without
// a percent sort last, ties break on display name.
func sortAchievements(achievements []models.Achievement) {
	sort.Slice(achievements, func(i, j int) bool {
		pi := achievements[i].GlobalPercent
		pj := achievements[j].GlobalPercent
		if pi == nil && pj == nil {
			return achievements[i].DisplayName < achievements[j].DisplayName
		}
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		return *pi > *pj
	})
}

// Knows reports whether appID is in userID's library or wishlist.
func (s *LibraryService) Knows(
	ctx context.Context,
	userID string,
	appID int,
) (bool, error) {
	_, err := s.steam.GetGameByID(ctx, appID, userID)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return false, err
	}

	items, err := s.steam.GetWishlist(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(items, func(w models.WishlistItem) bool {
		return w.ID == appID
	}), nil
}

// ListConsumption returns userID's played and wishlisted Steam games as
// consumption events, for the taste app. Playtime is judged against the
// user's own played games, so a top-quartile game is one they spent unusually
// long on.
func (s *LibraryService) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	games, err := s.steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, err
	}

	// A delisted game whose achievements a listed one took over is that same
	// game under an old id.
	played := slices.DeleteFunc(games, func(g models.Game) bool {
		return g.Playtime <= 0 || !g.InCompletionAverage
	})
	topQuartile := upperQuartile(played)

	events := make([]consumption.Event, 0, len(played))
	for _, g := range played {
		signal, reason := gameSignal(g, topQuartile)
		var at time.Time
		if g.LastPlayed != nil {
			at = *g.LastPlayed
		}
		events = append(events, consumption.Event{
			Domain:  consumption.DomainGames,
			ItemID:  strconv.Itoa(g.ID),
			Title:   g.Name,
			Creator: "",
			Tags:    nil,
			At:      at,
			Signal:  signal,
			Reason:  reason,
		})
	}

	wishlist, err := s.steam.GetWishlist(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, w := range wishlist {
		var at time.Time
		if w.AddedAt != nil {
			at = *w.AddedAt
		}
		events = append(events, consumption.Event{
			Domain:  consumption.DomainGames,
			ItemID:  strconv.Itoa(w.ID),
			Title:   w.Name,
			Creator: "",
			Tags:    nil,
			At:      at,
			Signal:  consumption.Positive,
			Reason:  "on your wishlist",
		})
	}
	return events, nil
}

func gameSignal(g models.Game, topQuartile int) (consumption.Signal, string) {
	rate, _ := strconv.ParseFloat(g.CompletionRate, 64)
	switch {
	case g.Favourite:
		return consumption.StrongPositive, "favourite"
	case rate >= fullCompletion:
		return consumption.StrongPositive, "every achievement"
	case g.Playtime < bouncedMinutes:
		return consumption.Negative, "under an hour played"
	case g.Playtime >= topQuartile:
		return consumption.Positive, "among your longest played"
	default:
		return consumption.Neutral, "played"
	}
}

// upperQuartile returns the 75th-percentile playtime of games.
func upperQuartile(games []models.Game) int {
	if len(games) == 0 {
		return 0
	}
	playtimes := make([]int, len(games))
	for i, g := range games {
		playtimes[i] = g.Playtime
	}
	slices.Sort(playtimes)
	const quartile = 3
	return playtimes[len(playtimes)*quartile/4]
}
