package backlog

import (
	"context"
	"errors"
	"sort"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ backlogv1connect.GamesServiceHandler = (*gamesConnectHandler)(nil)

type gamesConnectHandler struct {
	app *Backlog
}

func (h *gamesConnectHandler) GetSteam(
	ctx context.Context,
	req *connect.Request[backlogv1.GetSteamRequest],
) (*connect.Response[backlogv1.GetSteamResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	notStarted, err := h.app.Services.Steam.GetBacklog(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	inProgress, err := h.app.Services.Steam.GetInProgress(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	distribution, _, err := h.app.Services.Progress.GetCompletionRateDistribution(
		ctx,
		user.ID,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	completed, err := h.app.Services.Steam.GetCompleted(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	currentRate, err := h.app.Services.Progress.GetCurrentSteamCompletionRate(
		ctx,
		user.ID,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)

	labels, values, err := h.app.Services.Progress.GetByTypeIDAndDates(
		ctx, models.SteamTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&backlogv1.GetSteamResponse{
		Steam: &backlogv1.SteamResponse{
			NotStarted: protoGames(notStarted),
			InProgress: protoGames(inProgress),
			Completed:  protoGames(completed),
			//nolint:gosec // safe for domain counts
			TotalBacklog: int32(len(notStarted) + len(inProgress)),
			Distribution: convertIntSlice(distribution),
			CurrentRate:  currentRate,
			Labels:       labels,
			Values:       values,
			DateStart:    dateStart.Format(models.ProgressDateFormat),
			DateEnd:      dateEnd.Format(models.ProgressDateFormat),
		},
	}), nil
}

func (h *gamesConnectHandler) GetSteamGame(
	ctx context.Context,
	req *connect.Request[backlogv1.GetSteamGameRequest],
) (*connect.Response[backlogv1.GetSteamGameResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	gameID := int(req.Msg.GameId)

	game, err := h.app.Services.Steam.GetGameByID(ctx, gameID, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	achievements, err := h.app.Services.Steam.GetAchievementsForGame(
		ctx,
		gameID,
		user.ID,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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

	return connect.NewResponse(&backlogv1.GetSteamGameResponse{
		Data: &backlogv1.SteamGameResponse{
			Game:         protoGame(*game),
			Achievements: protoAchievements(achievements),
		},
	}), nil
}

func (h *gamesConnectHandler) GetSteamDistribution(
	ctx context.Context,
	req *connect.Request[backlogv1.GetSteamDistributionRequest],
) (*connect.Response[backlogv1.GetSteamDistributionResponse], error) {
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

	return connect.NewResponse(&backlogv1.GetSteamDistributionResponse{
		Data: &backlogv1.SteamDistributionResponse{
			Label: labels[bucket],
			Games: protoGames(bucketGames[bucket]),
		},
	}), nil
}

func (h *gamesConnectHandler) GetRecentlyActiveGames(
	ctx context.Context,
	_ *connect.Request[backlogv1.GetRecentlyActiveGamesRequest],
) (*connect.Response[backlogv1.GetRecentlyActiveGamesResponse], error) {
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

	return connect.NewResponse(&backlogv1.GetRecentlyActiveGamesResponse{
		Games: protoRecentGames(games),
	}), nil
}

// Proto conversion helpers for games

func protoGame(g models.Game) *backlogv1.Game {
	return &backlogv1.Game{
		Id:             int32(g.ID), //nolint:gosec // int32 safe for domain values
		Name:           g.Name,
		IsDelisted:     g.IsDelisted,
		CompletionRate: g.CompletionRate,
		Contribution:   g.Contribution,
		Playtime:       int32(g.Playtime), //nolint:gosec // safe for domain values
	}
}

func protoGames(games []models.Game) []*backlogv1.Game {
	result := make([]*backlogv1.Game, len(games))
	for i, g := range games {
		result[i] = protoGame(g)
	}
	return result
}

func protoRecentGames(games []models.RecentGame) []*backlogv1.RecentGame {
	result := make([]*backlogv1.RecentGame, len(games))
	for i, g := range games {
		result[i] = &backlogv1.RecentGame{
			Id:             int32(g.ID), //nolint:gosec // int32 safe for domain values
			Name:           g.Name,
			CompletionRate: g.CompletionRate,
			//nolint:gosec // safe for domain counts
			RecentUnlocks:  int32(g.RecentUnlocks),
			LastUnlockedAt: g.LastUnlocked.Format(models.ProgressDateFormat),
		}
	}
	return result
}

func protoAchievements(achievements []models.Achievement) []*backlogv1.Achievement {
	result := make([]*backlogv1.Achievement, len(achievements))
	for i, a := range achievements {
		result[i] = &backlogv1.Achievement{
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
