package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/games/internal/protomap"
	"tools.xdoubleu.com/apps/games/internal/services"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ gamesv1connect.GamesServiceHandler = (*Handler)(nil)

// Handler serves games.v1.GamesService.
type Handler struct {
	steam        *services.SteamService
	progress     *services.ProgressService
	library      *services.LibraryService
	integrations *services.IntegrationsService
}

// NewHandler returns a Handler backed by the games services.
func NewHandler(s *services.Services) *Handler {
	return &Handler{
		steam:        s.Steam,
		progress:     s.Progress,
		library:      s.Library,
		integrations: s.Integrations,
	}
}

func (h *Handler) GetSteam(
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

	dateStart, dateEnd := protomap.DateRange(req.DateStart, req.DateEnd)

	steam, err := h.SteamResponse(ctx, user.ID, dateStart, dateEnd)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetSteamResponse{Steam: steam}, nil
}

func (h *Handler) GetSteamGame(
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

	data, err := h.SteamGameResponse(ctx, user.ID, int(req.GameId))
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetSteamGameResponse{Data: data}, nil
}

func (h *Handler) RefreshSteamGame(
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

	if err := h.steam.SyncGame(ctx, user.ID, gameID); err != nil {
		return nil, connecttools.MapError(err)
	}

	data, err := h.SteamGameResponse(ctx, user.ID, gameID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.RefreshSteamGameResponse{Data: data}, nil
}

// SetGameFavourite flips a game's favourite flag and returns the game.
func (h *Handler) SetGameFavourite(
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

	err := h.steam.SetFavourite(
		ctx, user.ID, gameID, req.Favourite,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	game, err := h.steam.GetGameByID(ctx, gameID, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.SetGameFavouriteResponse{
		Game: protomap.Game(*game),
	}, nil
}

func (h *Handler) GetSteamDistribution(
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

	label, bucketGames, err := h.progress.GetDistributionBucket(
		ctx, user.ID, int(req.Bucket),
	)
	if err != nil {
		return nil, connecttools.MapError(
			err,
			connecttools.Code(services.ErrInvalidBucket, connect.CodeInvalidArgument),
		)
	}

	return &gamesv1.GetSteamDistributionResponse{
		Data: &gamesv1.SteamDistributionResponse{
			Label: label,
			Games: protomap.Games(bucketGames),
		},
	}, nil
}

func (h *Handler) GetRecentlyActiveGames(
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

	games, err := h.steam.GetRecentlyActive(ctx, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetRecentlyActiveGamesResponse{
		Games: protomap.RecentGames(games),
	}, nil
}

func (h *Handler) GetWishlist(
	ctx context.Context,
	_ *gamesv1.GetWishlistRequest,
) (*gamesv1.GetWishlistResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	items, err := h.steam.GetWishlist(ctx, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	games := protomap.Wishlist(items)
	return &gamesv1.GetWishlistResponse{Games: games}, nil
}
