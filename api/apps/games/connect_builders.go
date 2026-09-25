package games

import (
	"context"
	"sort"
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

// buildSteamResponse assembles a user's dashboard/library payload for GetSteam
// and BuildSharedSteam.
func (a *Games) buildSteamResponse(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*gamesv1.SteamResponse, error) {
	notStarted, err := a.Services.Steam.GetBacklog(ctx, userID)
	if err != nil {
		return nil, err
	}

	inProgress, err := a.Services.Steam.GetInProgress(ctx, userID)
	if err != nil {
		return nil, err
	}

	distribution, _, err := a.Services.Progress.GetCompletionRateDistribution(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	completed, err := a.Services.Steam.GetCompleted(ctx, userID)
	if err != nil {
		return nil, err
	}

	delisted, err := a.Services.Steam.GetDelisted(ctx, userID)
	if err != nil {
		return nil, err
	}

	currentRate, err := a.Services.Progress.GetCurrentSteamCompletionRate(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	labels, values, err := a.Services.Progress.GetByDates(
		ctx, userID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, err
	}

	return &gamesv1.SteamResponse{
		NotStarted: protoGames(notStarted),
		InProgress: protoGames(inProgress),
		Completed:  protoGames(completed),
		Delisted:   protoGames(delisted),
		//nolint:gosec // safe for domain counts
		TotalBacklog: int32(len(notStarted) + len(inProgress)),
		Distribution: convertIntSlice(distribution),
		CurrentRate:  currentRate,
		Labels:       labels,
		Values:       values,
		DateStart:    dateStart.Format(models.ProgressDateFormat),
		DateEnd:      dateEnd.Format(models.ProgressDateFormat),
	}, nil
}

// BuildSharedSteam assembles the public games dashboard's Steam payload plus
// the last library sync time ("" if never synced).
func (a *Games) BuildSharedSteam(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*gamesv1.SteamResponse, string, error) {
	steam, err := a.buildSteamResponse(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, "", err
	}

	lastSynced, err := a.Services.Steam.GetLastSyncedAt(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	lastSyncedAt := ""
	if lastSynced != nil {
		lastSyncedAt = lastSynced.Format(time.RFC3339)
	}

	return steam, lastSyncedAt, nil
}

// BuildSharedSteamGame is the exported buildSteamGameResponse for the
// dashboard app.
func (a *Games) BuildSharedSteamGame(
	ctx context.Context,
	userID string,
	gameID int,
) (*gamesv1.SteamGameResponse, error) {
	return a.buildSteamGameResponse(ctx, userID, gameID)
}

// BuildSharedRecentlyActiveGames returns recently-active games as proto for
// the dashboard app, which can't reference games' internal models.
func (a *Games) BuildSharedRecentlyActiveGames(
	ctx context.Context,
	userID string,
) ([]*gamesv1.RecentGame, error) {
	recentGames, err := a.Services.Steam.GetRecentlyActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	return protoRecentGames(recentGames), nil
}

// buildSteamGameResponse assembles one game's payload with achievements
// sorted by global percent.
func (a *Games) buildSteamGameResponse(
	ctx context.Context,
	userID string,
	gameID int,
) (*gamesv1.SteamGameResponse, error) {
	game, err := a.Services.Steam.GetGameByID(ctx, gameID, userID)
	if err != nil {
		return nil, err
	}

	achievements, err := a.Services.Steam.GetAchievementsForGame(
		ctx,
		gameID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	sortAchievements(achievements)

	return &gamesv1.SteamGameResponse{
		Game:         protoGame(*game),
		Achievements: protoAchievements(achievements),
	}, nil
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
