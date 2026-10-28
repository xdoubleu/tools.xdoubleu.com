package dashboard

import (
	"context"

	"connectrpc.com/connect/v2"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// publicConnectHandler serves the shareable dashboards without auth
// middleware; the share token alone authorizes.
type publicConnectHandler struct {
	app *Dashboard
}

var _ dashboardv1connect.PublicGamesDashboardServiceHandler = (*publicConnectHandler)(
	nil,
)

var _ dashboardv1connect.PublicReadingDashboardServiceHandler = (*publicConnectHandler)(
	nil,
)

// resolveToken maps a share token of the given kind to its owner; unknown or
// wrong-kind tokens are CodeNotFound, so it's no token oracle.
func (h *publicConnectHandler) resolveToken(
	ctx context.Context,
	token string,
	kind sharedmodels.DashboardKind,
) (string, string, error) {
	if token == "" {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			"unknown dashboard",
		)
	}
	userID, displayName, err := h.app.profileShares.ResolveToken(ctx, token, kind)
	if err != nil {
		return "", "", connecttools.MapError(err, connecttools.CodeWithMessage(
			database.ErrResourceNotFound, connect.CodeNotFound, "unknown dashboard",
		))
	}
	return userID, displayName, nil
}

func (h *publicConnectHandler) GetSharedSteam(
	ctx context.Context,
	req *dashboardv1.GetSharedSteamRequest,
) (*dashboardv1.GetSharedSteamResponse, error) {
	userID, displayName, err := h.resolveToken(
		ctx, req.Token, sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.DateStart, req.DateEnd)

	steam, lastSyncedAt, err := h.app.games.BuildSharedSteam(
		ctx,
		userID,
		dateStart,
		dateEnd,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedSteamResponse{
		Steam:        steam,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}, nil
}

func (h *publicConnectHandler) GetSharedSteamGame(
	ctx context.Context,
	req *dashboardv1.GetSharedSteamGameRequest,
) (*dashboardv1.GetSharedSteamGameResponse, error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Token,
		sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	data, err := h.app.games.BuildSharedSteamGame(ctx, userID, int(req.GameId))
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedSteamGameResponse{
		Data: data,
	}, nil
}

func (h *publicConnectHandler) GetSharedRecentlyActiveGames(
	ctx context.Context,
	req *dashboardv1.GetSharedRecentlyActiveGamesRequest,
) (*dashboardv1.GetSharedRecentlyActiveGamesResponse, error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Token,
		sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	recentGames, err := h.app.games.BuildSharedRecentlyActiveGames(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedRecentlyActiveGamesResponse{
		Games: recentGames,
	}, nil
}

func (h *publicConnectHandler) GetSharedLibrary(
	ctx context.Context,
	req *dashboardv1.GetSharedLibraryRequest,
) (*dashboardv1.GetSharedLibraryResponse, error) {
	userID, displayName, err := h.resolveToken(
		ctx, req.Token, sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	library, lastSyncedAt, err := h.app.books.BuildSharedLibrary(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedLibraryResponse{
		Library:      library,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}, nil
}

func (h *publicConnectHandler) GetSharedBooksProgress(
	ctx context.Context,
	req *dashboardv1.GetSharedBooksProgressRequest,
) (*dashboardv1.GetSharedBooksProgressResponse, error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Token,
		sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.DateStart, req.DateEnd)
	progress, err := h.app.books.BuildSharedProgress(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedBooksProgressResponse{
		Progress: progress,
	}, nil
}

func (h *publicConnectHandler) GetSharedFeedsSummary(
	ctx context.Context,
	req *dashboardv1.GetSharedFeedsSummaryRequest,
) (*dashboardv1.GetSharedFeedsSummaryResponse, error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Token,
		sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	sharedFeeds, err := h.app.feeds.BuildSharedFeeds(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedFeedsSummaryResponse{
		Feeds: protoSharedFeeds(sharedFeeds),
	}, nil
}
