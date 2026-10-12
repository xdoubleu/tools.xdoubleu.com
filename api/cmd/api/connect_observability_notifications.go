package main

import (
	"context"

	"connectrpc.com/connect/v2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/repositories"
)

// Notification settings control which sources jobs.WeeklyDigestJob may email
// the admin about. They're global, so only admins may change them; anyone can
// read the toggles, but only admins see the admin's address.

func (h *obsConnectHandler) GetNotificationSettings(
	ctx context.Context,
	_ *observabilityv1.GetNotificationSettingsRequest,
) (*observabilityv1.GetNotificationSettingsResponse, error) {
	resp, err := h.notificationSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	if requireAdmin(ctx) != nil {
		resp.AdminEmail = ""
	}
	return resp, nil
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
	req *observabilityv1.UpdateNotificationSettingsRequest,
) (*observabilityv1.UpdateNotificationSettingsResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	source := repositories.NotificationSource(req.GetSourceKey())
	if err := h.app.notificationSettingsRepo.SetEnabled(
		ctx, source, req.GetEnabled(),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &observabilityv1.UpdateNotificationSettingsResponse{}, nil
}
