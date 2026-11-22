package connect

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/dashboard/internal/services"
	"tools.xdoubleu.com/apps/feeds"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/connecttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// GamesShared builds the games dashboard payloads.
type GamesShared interface {
	BuildSharedSteam(
		ctx context.Context, userID string, dateStart, dateEnd time.Time,
	) (*gamesv1.SteamResponse, string, error)
	BuildSharedSteamGame(
		ctx context.Context, userID string, gameID int,
	) (*gamesv1.SteamGameResponse, error)
	BuildSharedRecentlyActiveGames(
		ctx context.Context, userID string,
	) ([]*gamesv1.RecentGame, error)
}

// BooksShared builds the reading dashboard's library and progress payloads.
type BooksShared interface {
	BuildSharedLibrary(
		ctx context.Context, userID string,
	) (*booksv1.LibraryResponse, string, error)
	BuildSharedProgress(
		ctx context.Context, userID string, dateStart, dateEnd time.Time,
	) (*booksv1.BooksProgressResponse, error)
}

// FeedsShared builds the reading dashboard's feeds payload.
type FeedsShared interface {
	BuildSharedFeeds(ctx context.Context, userID string) ([]feeds.SharedFeed, error)
}

// PublicHandler serves the shareable dashboards without auth middleware; the
// share token alone authorizes.
type PublicHandler struct {
	shares *services.ShareService
	games  GamesShared
	books  BooksShared
	feeds  FeedsShared
}

var _ dashboardv1connect.PublicGamesDashboardServiceHandler = (*PublicHandler)(nil)

var _ dashboardv1connect.PublicReadingDashboardServiceHandler = (*PublicHandler)(nil)

// NewPublicHandler returns a PublicHandler over the share service and the
// apps' dashboard builders.
func NewPublicHandler(
	shares *services.ShareService,
	games GamesShared,
	books BooksShared,
	feeds FeedsShared,
) *PublicHandler {
	return &PublicHandler{shares: shares, games: games, books: books, feeds: feeds}
}

// resolveToken maps a share token of the given kind to its owner; unknown or
// wrong-kind tokens are CodeNotFound.
func (h *PublicHandler) resolveToken(
	ctx context.Context,
	token string,
	kind sharedmodels.DashboardKind,
) (string, string, error) {
	userID, displayName, err := h.shares.Resolve(ctx, token, kind)
	if errors.Is(err, services.ErrUnknownDashboard) {
		return "", "", connect.NewError(connect.CodeNotFound, "unknown dashboard")
	}
	if err != nil {
		return "", "", connecttools.MapError(err)
	}
	return userID, displayName, nil
}

func (h *PublicHandler) GetSharedSteam(
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

	steam, lastSyncedAt, err := h.games.BuildSharedSteam(
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

func (h *PublicHandler) GetSharedSteamGame(
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

	data, err := h.games.BuildSharedSteamGame(ctx, userID, int(req.GameId))
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedSteamGameResponse{
		Data: data,
	}, nil
}

func (h *PublicHandler) GetSharedRecentlyActiveGames(
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

	recentGames, err := h.games.BuildSharedRecentlyActiveGames(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedRecentlyActiveGamesResponse{
		Games: recentGames,
	}, nil
}

func (h *PublicHandler) GetSharedLibrary(
	ctx context.Context,
	req *dashboardv1.GetSharedLibraryRequest,
) (*dashboardv1.GetSharedLibraryResponse, error) {
	userID, displayName, err := h.resolveToken(
		ctx, req.Token, sharedmodels.DashboardKindReading,
	)
	if err != nil {
		return nil, err
	}

	library, lastSyncedAt, err := h.books.BuildSharedLibrary(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedLibraryResponse{
		Library:      library,
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}, nil
}

func (h *PublicHandler) GetSharedBooksProgress(
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
	progress, err := h.books.BuildSharedProgress(ctx, userID, dateStart, dateEnd)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedBooksProgressResponse{
		Progress: progress,
	}, nil
}

func (h *PublicHandler) GetSharedFeedsSummary(
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

	sharedFeeds, err := h.feeds.BuildSharedFeeds(ctx, userID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &dashboardv1.GetSharedFeedsSummaryResponse{
		Feeds: protoSharedFeeds(sharedFeeds),
	}, nil
}

// dateFormat matches games/books' ProgressDateFormat.
const dateFormat = "2006-01-02"

func parseDateRangeFromStrings(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if dateStart != "" {
		if t, err := time.Parse(dateFormat, dateStart); err == nil {
			start = t
		}
	}
	if dateEnd != "" {
		if t, err := time.Parse(dateFormat, dateEnd); err == nil {
			end = t
		}
	}
	return start, end
}

func protoSharedFeeds(sharedFeeds []feeds.SharedFeed) []*dashboardv1.SharedFeed {
	proto := make([]*dashboardv1.SharedFeed, 0, len(sharedFeeds))
	for _, f := range sharedFeeds {
		proto = append(proto, &dashboardv1.SharedFeed{
			Title: f.Title,
			Url:   f.URL,
		})
	}
	return proto
}
