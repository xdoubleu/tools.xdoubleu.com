package games

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/games/internal/models"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ gamesv1connect.GamesServiceHandler = (*gamesConnectHandler)(nil)

type gamesConnectHandler struct {
	app *Games
}

func (h *gamesConnectHandler) GetSteam(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSteamRequest],
) (*connect.Response[gamesv1.GetSteamResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)

	steam, err := h.app.buildSteamResponse(ctx, user.ID, dateStart, dateEnd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetSteamResponse{Steam: steam}), nil
}

func (h *gamesConnectHandler) GetSteamGame(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSteamGameRequest],
) (*connect.Response[gamesv1.GetSteamGameResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	data, err := h.app.buildSteamGameResponse(ctx, user.ID, int(req.Msg.GameId))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetSteamGameResponse{Data: data}), nil
}

func (h *gamesConnectHandler) RefreshSteamGame(
	ctx context.Context,
	req *connect.Request[gamesv1.RefreshSteamGameRequest],
) (*connect.Response[gamesv1.RefreshSteamGameResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	gameID := int(req.Msg.GameId)

	if err := h.app.Services.Steam.SyncGame(ctx, user.ID, gameID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	data, err := h.app.buildSteamGameResponse(ctx, user.ID, gameID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.RefreshSteamGameResponse{Data: data}), nil
}

// SetGameFavourite flips the user-set favourite flag on a game and returns
// the updated game.
func (h *gamesConnectHandler) SetGameFavourite(
	ctx context.Context,
	req *connect.Request[gamesv1.SetGameFavouriteRequest],
) (*connect.Response[gamesv1.SetGameFavouriteResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	gameID := int(req.Msg.GameId)

	err := h.app.Services.Steam.SetFavourite(
		ctx, user.ID, gameID, req.Msg.Favourite,
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	game, err := h.app.Services.Steam.GetGameByID(ctx, gameID, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.SetGameFavouriteResponse{
		Game: protoGame(*game),
	}), nil
}

func (h *gamesConnectHandler) GetSteamDistribution(
	ctx context.Context,
	req *connect.Request[gamesv1.GetSteamDistributionRequest],
) (*connect.Response[gamesv1.GetSteamDistributionResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	labels := distributionLabels()
	bucket := int(req.Msg.Bucket)

	if bucket < 0 || bucket >= len(labels) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid bucket index"),
		)
	}

	_, bucketGames, err := h.app.Services.Progress.GetCompletionRateDistribution(
		ctx,
		user.ID,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetSteamDistributionResponse{
		Data: &gamesv1.SteamDistributionResponse{
			Label: labels[bucket],
			Games: protoGames(bucketGames[bucket]),
		},
	}), nil
}

func (h *gamesConnectHandler) GetRecentlyActiveGames(
	ctx context.Context,
	_ *connect.Request[gamesv1.GetRecentlyActiveGamesRequest],
) (*connect.Response[gamesv1.GetRecentlyActiveGamesResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	games, err := h.app.Services.Steam.GetRecentlyActive(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetRecentlyActiveGamesResponse{
		Games: protoRecentGames(games),
	}), nil
}

// Proto conversion helpers for games

func protoGame(g models.Game) *gamesv1.Game {
	return &gamesv1.Game{
		Id:             int32(g.ID), //nolint:gosec // int32 safe for domain values
		Name:           g.Name,
		IsDelisted:     g.IsDelisted,
		CompletionRate: g.CompletionRate,
		Contribution:   g.Contribution,
		Playtime:       int32(g.Playtime), //nolint:gosec // safe for domain values
		ImageUrl:       g.ImageURL,
		LastSyncedAt:   g.LastSyncedAt.Format(time.RFC3339),
		Favourite:      g.Favourite,
	}
}

func protoGames(games []models.Game) []*gamesv1.Game {
	result := make([]*gamesv1.Game, len(games))
	for i, g := range games {
		result[i] = protoGame(g)
	}
	return result
}

func protoRecentGames(games []models.RecentGame) []*gamesv1.RecentGame {
	result := make([]*gamesv1.RecentGame, len(games))
	for i, g := range games {
		result[i] = &gamesv1.RecentGame{
			Id:             int32(g.ID), //nolint:gosec // int32 safe for domain values
			Name:           g.Name,
			CompletionRate: g.CompletionRate,
			//nolint:gosec // safe for domain counts
			RecentUnlocks:  int32(g.RecentUnlocks),
			LastUnlockedAt: g.LastUnlocked.Format(models.ProgressDateFormat),
			ImageUrl:       g.ImageURL,
		}
	}
	return result
}

func protoAchievements(achievements []models.Achievement) []*gamesv1.Achievement {
	result := make([]*gamesv1.Achievement, len(achievements))
	for i, a := range achievements {
		result[i] = &gamesv1.Achievement{
			Name:          a.Name,
			DisplayName:   a.DisplayName,
			Description:   a.Description,
			IconUrl:       a.IconURL,
			Achieved:      a.Achieved,
			GlobalPercent: a.GlobalPercent,
		}
	}
	return result
}

func convertIntSlice(ints []int) []int32 {
	result := make([]int32, len(ints))
	for i, v := range ints {
		result[i] = int32(v) //nolint:gosec // int32 safe for domain values
	}
	return result
}
