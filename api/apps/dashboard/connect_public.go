package dashboard

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// publicConnectHandler serves the read-only shareable dashboards. It is
// registered WITHOUT auth middleware: requests are authorized solely by the
// opaque dashboard share token, resolved to the owning user here.
type publicConnectHandler struct {
	app *Dashboard
}

var _ dashboardv1connect.PublicGamesDashboardServiceHandler = (*publicConnectHandler)(
	nil,
)

var _ dashboardv1connect.PublicReadingDashboardServiceHandler = (*publicConnectHandler)(
	nil,
)

// resolveToken maps a share token to the owning user ID and display name,
// scoped to the given dashboard kind; unknown or wrong-kind tokens surface
// as CodeNotFound to avoid acting as a token oracle.
func (h *publicConnectHandler) resolveToken(
	ctx context.Context,
	token string,
	kind sharedmodels.DashboardKind,
) (string, string, error) {
	if token == "" {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown dashboard"),
		)
	}
	userID, displayName, err := h.app.profileShares.ResolveToken(ctx, token, kind)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown dashboard"),
		)
	}
	if err != nil {
		return "", "", connect.NewError(connect.CodeInternal, err)
	}
	return userID, displayName, nil
}

func (h *publicConnectHandler) GetSharedSteam(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedSteamRequest],
) (*connect.Response[dashboardv1.GetSharedSteamResponse], error) {
	userID, displayName, err := h.resolveToken(
		ctx, req.Msg.Token, sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)

	steam, lastSyncedAt, err := h.app.games.BuildSharedSteam(
		ctx,
		userID,
		dateStart,
		dateEnd,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedSteamResponse{
		Steam:        steam,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}), nil
}

func (h *publicConnectHandler) GetSharedSteamGame(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedSteamGameRequest],
) (*connect.Response[dashboardv1.GetSharedSteamGameResponse], error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Msg.Token,
		sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	data, err := h.app.games.BuildSharedSteamGame(ctx, userID, int(req.Msg.GameId))
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedSteamGameResponse{
		Data: data,
	}), nil
}

func (h *publicConnectHandler) GetSharedRecentlyActiveGames(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedRecentlyActiveGamesRequest],
) (*connect.Response[dashboardv1.GetSharedRecentlyActiveGamesResponse], error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Msg.Token,
		sharedmodels.DashboardKindGames,
	)
	if err != nil {
		return nil, err
	}

	recentGames, err := h.app.games.BuildSharedRecentlyActiveGames(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedRecentlyActiveGamesResponse{
		Games: recentGames,
	}), nil
}

func (h *publicConnectHandler) GetSharedLibrary(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedLibraryRequest],
) (*connect.Response[dashboardv1.GetSharedLibraryResponse], error) {
	userID, displayName, err := h.resolveToken(
		ctx, req.Msg.Token, sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	library, lastSyncedAt, err := h.app.books.BuildSharedLibrary(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedLibraryResponse{
		Library:      library,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}), nil
}

func (h *publicConnectHandler) GetSharedBooksProgress(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedBooksProgressRequest],
) (*connect.Response[dashboardv1.GetSharedBooksProgressResponse], error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Msg.Token,
		sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)
	progress, err := h.app.books.BuildSharedProgress(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedBooksProgressResponse{
		Progress: progress,
	}), nil
}

func (h *publicConnectHandler) GetSharedFeedsSummary(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetSharedFeedsSummaryRequest],
) (*connect.Response[dashboardv1.GetSharedFeedsSummaryResponse], error) {
	userID, _, err := h.resolveToken(
		ctx,
		req.Msg.Token,
		sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	sharedFeeds, err := h.app.feeds.BuildSharedFeeds(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.GetSharedFeedsSummaryResponse{
		Feeds: protoSharedFeeds(sharedFeeds),
	}), nil
}
