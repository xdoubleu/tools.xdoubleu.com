package games

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// publicConnectHandler serves the read-only shareable profile RPCs. It is
// registered WITHOUT auth middleware: requests are authorized solely by the
// profile share token, resolved to the owning user here.
type publicConnectHandler struct {
	app *Games
}

var _ gamesv1connect.PublicGamesServiceHandler = (*publicConnectHandler)(nil)

// resolveToken maps a share token to the owning user ID and display name,
// scoped to the games app; unknown or wrong-app tokens surface as
// CodeNotFound to avoid acting as a token oracle.
func (h *publicConnectHandler) resolveToken(
	ctx context.Context,
	token string,
) (string, string, error) {
	if token == "" {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	userID, displayName, err := h.app.profileShares.ResolveToken(
		ctx, token, sharedmodels.ProfileAppGames,
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	if err != nil {
		return "", "", connect.NewError(connect.CodeInternal, err)
	}
	return userID, displayName, nil
}

func (h *publicConnectHandler) GetSharedSteam(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSharedSteamRequest],
) (*connect.Response[gamesv1.GetSharedSteamResponse], error) {
	userID, displayName, err := h.resolveToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)

	steam, err := h.app.buildSteamResponse(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	lastSynced, err := h.app.Services.Steam.GetLastSyncedAt(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	lastSyncedAt := ""
	if lastSynced != nil {
		lastSyncedAt = lastSynced.Format(time.RFC3339)
	}

	return connect.NewResponse(&gamesv1.GetSharedSteamResponse{
		Steam:        steam,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}), nil
}

func (h *publicConnectHandler) GetSharedSteamGame(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSharedSteamGameRequest],
) (*connect.Response[gamesv1.GetSharedSteamGameResponse], error) {
	userID, _, err := h.resolveToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, err
	}

	data, err := h.app.buildSteamGameResponse(ctx, userID, int(req.Msg.GameId))
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetSharedSteamGameResponse{
		Data: data,
	}), nil
}

func (h *publicConnectHandler) GetSharedRecentlyActiveGames(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSharedRecentlyActiveGamesRequest],
) (*connect.Response[gamesv1.GetSharedRecentlyActiveGamesResponse], error) {
	userID, _, err := h.resolveToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, err
	}

	games, err := h.app.Services.Steam.GetRecentlyActive(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetSharedRecentlyActiveGamesResponse{
		Games: protoRecentGames(games),
	}), nil
}
