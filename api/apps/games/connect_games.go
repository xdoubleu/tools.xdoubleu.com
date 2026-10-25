package games

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/games/internal/services"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ gamesv1connect.GamesServiceHandler = (*gamesConnectHandler)(nil)

type gamesConnectHandler struct {
	app *Games
}

func (h *gamesConnectHandler) GetSteam(
	ctx context.Context,
	req *gamesv1.GetSteamRequest,
) (*gamesv1.GetSteamResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.DateStart, req.DateEnd)

	steam, err := h.app.buildSteamResponse(ctx, user.ID, dateStart, dateEnd)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetSteamResponse{Steam: steam}, nil
}

func (h *gamesConnectHandler) GetSteamGame(
	ctx context.Context,
	req *gamesv1.GetSteamGameRequest,
) (*gamesv1.GetSteamGameResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	data, err := h.app.buildSteamGameResponse(ctx, user.ID, int(req.GameId))
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetSteamGameResponse{Data: data}, nil
}

func (h *gamesConnectHandler) RefreshSteamGame(
	ctx context.Context,
	req *gamesv1.RefreshSteamGameRequest,
) (*gamesv1.RefreshSteamGameResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	gameID := int(req.GameId)

	if err := h.app.Services.Steam.SyncGame(ctx, user.ID, gameID); err != nil {
		return nil, connecttools.MapError(err)
	}

	data, err := h.app.buildSteamGameResponse(ctx, user.ID, gameID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.RefreshSteamGameResponse{Data: data}, nil
}

// SetGameFavourite flips a game's favourite flag and returns the game.
func (h *gamesConnectHandler) SetGameFavourite(
	ctx context.Context,
	req *gamesv1.SetGameFavouriteRequest,
) (*gamesv1.SetGameFavouriteResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	gameID := int(req.GameId)

	err := h.app.Services.Steam.SetFavourite(
		ctx, user.ID, gameID, req.Favourite,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	game, err := h.app.Services.Steam.GetGameByID(ctx, gameID, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.SetGameFavouriteResponse{
		Game: protoGame(*game),
	}, nil
}

func (h *gamesConnectHandler) GetSteamDistribution(
	ctx context.Context,
	req *gamesv1.GetSteamDistributionRequest,
) (*gamesv1.GetSteamDistributionResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	label, bucketGames, err := h.app.Services.Progress.GetDistributionBucket(
		ctx, user.ID, int(req.Bucket),
	)
	if errors.Is(err, services.ErrInvalidBucket) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetSteamDistributionResponse{
		Data: &gamesv1.SteamDistributionResponse{
			Label: label,
			Games: protoGames(bucketGames),
		},
	}, nil
}

func (h *gamesConnectHandler) GetRecentlyActiveGames(
	ctx context.Context,
	_ *gamesv1.GetRecentlyActiveGamesRequest,
) (*gamesv1.GetRecentlyActiveGamesResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	games, err := h.app.Services.Steam.GetRecentlyActive(ctx, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetRecentlyActiveGamesResponse{
		Games: protoRecentGames(games),
	}, nil
}

func protoGame(g models.Game) *gamesv1.Game {
	return &gamesv1.Game{
		Id:                  int32(g.ID), //nolint:gosec // int32 safe for domain values
		Name:                g.Name,
		IsDelisted:          g.IsDelisted,
		InCompletionAverage: g.InCompletionAverage,
		CompletionRate:      g.CompletionRate,
		Contribution:        g.Contribution,
		Playtime:            int32(g.Playtime), //nolint:gosec // safe for domain values
		ImageUrl:            g.ImageURL,
		LastSyncedAt:        g.LastSyncedAt.Format(time.RFC3339),
		Favourite:           g.Favourite,
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
			ImageUrl:       g.ImageURL,
			LastPlayedAt:   g.LastPlayed.Format(models.ProgressDateFormat),
			Playtime:       int32(g.Playtime), //nolint:gosec // safe for domain values
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
