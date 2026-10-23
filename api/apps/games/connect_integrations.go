package games

import (
	"context"
	"regexp"

	"connectrpc.com/connect/v2"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const steamUserIDMaxLen = 20

var numericOrEmptyRe = regexp.MustCompile(`^\d*$`)

func (h *gamesConnectHandler) GetIntegrations(
	ctx context.Context,
	_ *gamesv1.GetIntegrationsRequest,
) (*gamesv1.GetIntegrationsResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	integrations, err := h.app.GetIntegrations(ctx, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.GetIntegrationsResponse{
		Integrations: &gamesv1.Integrations{
			SteamUserId: integrations.SteamUserID,
		},
	}, nil
}

func (h *gamesConnectHandler) SaveIntegrations(
	ctx context.Context,
	req *gamesv1.SaveIntegrationsRequest,
) (*gamesv1.SaveIntegrationsResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}

	if req.Integrations == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"integrations required",
		)
	}

	i := req.Integrations
	if !numericOrEmptyRe.MatchString(i.SteamUserId) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"steam_user_id must be numeric",
		)
	}
	if len(i.SteamUserId) > steamUserIDMaxLen {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"steam_user_id too long",
		)
	}

	if err := h.app.SaveIntegrations(ctx, user.ID, Integrations{
		SteamUserID: i.SteamUserId,
	}); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &gamesv1.SaveIntegrationsResponse{}, nil
}
