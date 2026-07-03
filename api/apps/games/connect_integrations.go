package games

import (
	"context"
	"errors"
	"regexp"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const steamUserIDMaxLen = 20

var numericOrEmptyRe = regexp.MustCompile(`^\d*$`)

func (h *gamesConnectHandler) GetIntegrations(
	ctx context.Context,
	_ *connect.Request[gamesv1.GetIntegrationsRequest],
) (*connect.Response[gamesv1.GetIntegrationsResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	integrations, err := h.app.GetIntegrations(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.GetIntegrationsResponse{
		Integrations: &gamesv1.Integrations{
			SteamUserId: integrations.SteamUserID,
		},
	}), nil
}

func (h *gamesConnectHandler) SaveIntegrations(
	ctx context.Context,
	req *connect.Request[gamesv1.SaveIntegrationsRequest],
) (*connect.Response[gamesv1.SaveIntegrationsResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	if req.Msg.Integrations == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("integrations required"),
		)
	}

	i := req.Msg.Integrations
	if !numericOrEmptyRe.MatchString(i.SteamUserId) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("steam_user_id must be numeric"),
		)
	}
	if len(i.SteamUserId) > steamUserIDMaxLen {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("steam_user_id too long"),
		)
	}

	if err := h.app.SaveIntegrations(ctx, user.ID, Integrations{
		SteamUserID: i.SteamUserId,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamesv1.SaveIntegrationsResponse{}), nil
}
