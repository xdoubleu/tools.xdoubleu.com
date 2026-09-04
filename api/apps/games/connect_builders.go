package games

import (
	"context"
	"sort"
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

// buildSteamResponse assembles the dashboard/library payload for a user. It
// is shared by the authenticated GetSteam RPC and BuildSharedSteam (used by
// the dashboard app's public games dashboard, see api/apps/dashboard).
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

// BuildSharedSteam assembles the games dashboard's Steam payload for a user,
// plus their most recent Steam library sync time (empty string when the
// library has never been synced). This is the one exported entry point the
// dashboard app (api/apps/dashboard) uses to build its public games
// dashboard, keeping games' own achievement/progress logic un-duplicated.
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

// BuildSharedSteamGame assembles a single game's payload for the dashboard
// app's public games dashboard (api/apps/dashboard) — the exported
// counterpart of buildSteamGameResponse for callers outside this package.
func (a *Games) BuildSharedSteamGame(
	ctx context.Context,
	userID string,
	gameID int,
) (*gamesv1.SteamGameResponse, error) {
	return a.buildSteamGameResponse(ctx, userID, gameID)
}

// BuildSharedRecentlyActiveGames assembles the recently-active games list for
// the dashboard app's public games dashboard. GetRecentlyActive itself
// returns games' internal domain model (apps/games/internal/models), which
// callers outside this package cannot reference — this wrapper is the
// exported boundary that converts to the proto type instead.
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

// buildSteamGameResponse assembles a single game's payload with its
// achievements sorted by global completion percent (most common first).
// Shared by GetSteamGame, RefreshSteamGame, and BuildSharedSteamGame.
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
