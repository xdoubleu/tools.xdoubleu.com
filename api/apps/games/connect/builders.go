package connect

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/games/internal/protomap"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

// SteamResponse assembles a user's dashboard/library payload for GetSteam
// and the public games dashboard.
func (h *Handler) SteamResponse(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*gamesv1.SteamResponse, error) {
	o, err := h.library.GetSteam(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, err
	}

	return protomap.Steam(o), nil
}

// LastSyncedAt is the last library sync time as RFC 3339 ("" if never
// synced).
func (h *Handler) LastSyncedAt(ctx context.Context, userID string) (string, error) {
	lastSynced, err := h.steam.GetLastSyncedAt(ctx, userID)
	if err != nil || lastSynced == nil {
		return "", err
	}
	return lastSynced.Format(time.RFC3339), nil
}

// SteamGameResponse assembles one game's payload with achievements sorted by
// global percent.
func (h *Handler) SteamGameResponse(
	ctx context.Context,
	userID string,
	gameID int,
) (*gamesv1.SteamGameResponse, error) {
	d, err := h.library.GetSteamGame(ctx, userID, gameID)
	if err != nil {
		return nil, err
	}
	return protomap.SteamGame(d), nil
}

// RecentGames returns recently-active games as proto for the dashboard app.
func (h *Handler) RecentGames(
	ctx context.Context,
	userID string,
) ([]*gamesv1.RecentGame, error) {
	recent, err := h.steam.GetRecentlyActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	return protomap.RecentGames(recent), nil
}
