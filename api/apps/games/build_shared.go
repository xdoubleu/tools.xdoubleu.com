package games

import (
	"context"
	"time"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

// BuildSharedSteam assembles the public games dashboard's Steam payload plus
// the last library sync time ("" if never synced).
func (a *Games) BuildSharedSteam(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*gamesv1.SteamResponse, string, error) {
	h := a.connectHandler()
	steam, err := h.SteamResponse(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, "", err
	}

	lastSyncedAt, err := h.LastSyncedAt(ctx, userID)
	if err != nil {
		return nil, "", err
	}

	return steam, lastSyncedAt, nil
}

// BuildSharedSteamGame assembles one game's payload for the dashboard app.
func (a *Games) BuildSharedSteamGame(
	ctx context.Context,
	userID string,
	gameID int,
) (*gamesv1.SteamGameResponse, error) {
	return a.connectHandler().SteamGameResponse(ctx, userID, gameID)
}

// BuildSharedRecentlyActiveGames returns recently-active games as proto for
// the dashboard app, which can't reference games' internal models.
func (a *Games) BuildSharedRecentlyActiveGames(
	ctx context.Context,
	userID string,
) ([]*gamesv1.RecentGame, error) {
	return a.connectHandler().RecentGames(ctx, userID)
}
