package main

import (
	"context"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/repositories"
)

// Notification settings control which sources jobs.WeeklyDigestJob may email
// the admin about. They're global, so only admins may change them; anyone can
// read the toggles, but only admins see the admin's address.

func (h *obsConnectHandler) GetNotificationSettings(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetNotificationSettingsRequest],
) (*connect.Response[observabilityv1.GetNotificationSettingsResponse], error) {
	resp, err := h.notificationSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if requireAdmin(ctx) != nil {
		resp.AdminEmail = ""
	}
	return connect.NewResponse(resp), nil
}

// notificationSettings is shared by the Connect handler and MCP tool.
func (h *obsConnectHandler) notificationSettings(
	ctx context.Context,
) (*observabilityv1.GetNotificationSettingsResponse, error) {
	settings, err := h.app.notificationSettingsRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	protoSettings := make([]*observabilityv1.NotificationSetting, len(settings))
	for i, s := range settings {
		protoSettings[i] = &observabilityv1.NotificationSetting{
			SourceKey: string(s.SourceKey),
			Enabled:   s.Enabled,
		}
	}

	return &observabilityv1.GetNotificationSettingsResponse{
		Settings:   protoSettings,
		AdminEmail: h.app.config.NotifyEmailTo,
	}, nil
}

func (h *obsConnectHandler) UpdateNotificationSettings(
	ctx context.Context,
	req *connect.Request[observabilityv1.UpdateNotificationSettingsRequest],
) (*connect.Response[observabilityv1.UpdateNotificationSettingsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	source := repositories.NotificationSource(req.Msg.GetSourceKey())
	if err := h.app.notificationSettingsRepo.SetEnabled(
		ctx, source, req.Msg.GetEnabled(),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(
		&observabilityv1.UpdateNotificationSettingsResponse{},
	), nil
}
