package main

import (
	"context"
	"errors"
	"regexp"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/backlog"
	settingsv1 "tools.xdoubleu.com/gen/settings/v1"
	"tools.xdoubleu.com/gen/settings/v1/settingsv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

const (
	steamAPIKeyMaxLen     = 64
	steamUserIDMaxLen     = 20
	hardcoverAPIKeyMaxLen = 256
)

var numericOrEmptyRe = regexp.MustCompile(`^\d*$`)

type settingsConnectHandler struct {
	app *Application
}

var _ settingsv1connect.SettingsServiceHandler = (*settingsConnectHandler)(nil)

func (h *settingsConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

func (h *settingsConnectHandler) GetSettings(
	ctx context.Context,
	_ *connect.Request[settingsv1.GetSettingsRequest],
) (*connect.Response[settingsv1.GetSettingsResponse], error) {
	userID := h.userID(ctx)

	integrations, err := h.app.backlog.GetIntegrations(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&settingsv1.GetSettingsResponse{
		Integrations: &settingsv1.Integrations{
			SteamApiKey:     integrations.SteamAPIKey,
			SteamUserId:     integrations.SteamUserID,
			HardcoverApiKey: integrations.HardcoverAPIKey,
		},
	}), nil
}

func (h *settingsConnectHandler) SaveSettings(
	ctx context.Context,
	req *connect.Request[settingsv1.SaveSettingsRequest],
) (*connect.Response[settingsv1.SaveSettingsResponse], error) {
	userID := h.userID(ctx)

	if req.Msg.Integrations == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("integrations required"),
		)
	}

	i := req.Msg.Integrations
	if len(i.SteamApiKey) > steamAPIKeyMaxLen {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("steam_api_key too long"),
		)
	}
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
	if len(i.HardcoverApiKey) > hardcoverAPIKeyMaxLen {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("hardcover_api_key too long"),
		)
	}

	integrations := backlog.Integrations{
		SteamAPIKey:     i.SteamApiKey,
		SteamUserID:     i.SteamUserId,
		HardcoverAPIKey: i.HardcoverApiKey,
	}

	if err := h.app.backlog.SaveIntegrations(ctx, userID, integrations); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&settingsv1.SaveSettingsResponse{}), nil
}
