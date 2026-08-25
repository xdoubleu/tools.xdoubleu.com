package main

import (
	"context"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/repositories"
)

// GetNotificationSettings and UpdateNotificationSettings let any
// authenticated user see and toggle which sources (Sentry issues, failing
// dependency PRs, unhealthy feeds) are currently allowed to email an admin,
// via jobs.IssueNotifierJob/jobs.WeeklyDigestJob (issue #1214) — deliberately
// not gated by requireAdmin like the rest of ObservabilityService, since the
// unhealthy-feeds toggle is surfaced from the feeds app rather than
// monitoring and shouldn't require admin access to reach (issue #1228),
// mirroring dashboard.v1.DashboardService's own normal-Access carve-out.

func (h *obsConnectHandler) GetNotificationSettings(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetNotificationSettingsRequest],
) (*connect.Response[observabilityv1.GetNotificationSettingsResponse], error) {
	resp, err := h.notificationSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// notificationSettings is shared by the Connect handler above and the
// get_notification_settings MCP tool.
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
		Settings: protoSettings,
	}, nil
}

func (h *obsConnectHandler) UpdateNotificationSettings(
	ctx context.Context,
	req *connect.Request[observabilityv1.UpdateNotificationSettingsRequest],
) (*connect.Response[observabilityv1.UpdateNotificationSettingsResponse], error) {
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
