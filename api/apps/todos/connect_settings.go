package todos

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *settingsConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	return u.ID
}

// resolveWorkspaceID returns the workspace targeted by a request: the
// explicitly provided ID when it parses as a UUID, otherwise the user's
// active workspace.
func (h *settingsConnectHandler) resolveWorkspaceID(
	ctx context.Context,
	userID string,
	requested string,
) (*uuid.UUID, error) {
	if requested != "" {
		if wsID, err := uuid.Parse(requested); err == nil {
			return &wsID, nil
		}
	}
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, err
	}
	return wsCtx.Settings.ActiveWorkspaceID, nil
}

func (h *settingsConnectHandler) GetSettings(
	ctx context.Context,
	_ *connect.Request[todosv1.GetSettingsRequest],
) (*connect.Response[todosv1.GetSettingsResponse], error) {
	userID := h.userID(ctx)

	agg, err := h.app.services.Settings.GetSettings(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}

	protoPresets := make([]*todosv1.LabelPreset, len(agg.LabelPresets.Labels))
	for i, p := range agg.LabelPresets.Labels {
		protoPresets[i] = &todosv1.LabelPreset{Value: p.Value, Color: p.Color}
	}

	protoPatterns := make([]*todosv1.URLPattern, len(agg.URLPatterns))
	for i, p := range agg.URLPatterns {
		protoPatterns[i] = &todosv1.URLPattern{
			Id:           p.ID.String(),
			UserId:       p.UserID,
			UrlPrefix:    p.URLPrefix,
			PlatformName: p.PlatformName,
			Label:        p.Label,
			Shortcut:     p.Shortcut,
			SortOrder: int32( //nolint:gosec // int32 safe for domain values
				p.SortOrder,
			),
		}
	}

	protoPolicies := make([]*todosv1.Policy, len(agg.Policies))
	for i, p := range agg.Policies {
		protoPolicies[i] = &todosv1.Policy{
			Id:          p.ID.String(),
			OwnerUserId: p.OwnerUserID,
			Text:        p.Text,
			ReappearAfterHours: int32( //nolint:gosec // int32 safe for domain values
				p.ReappearAfterHours,
			),
			SortOrder: int32( //nolint:gosec // int32 safe for domain values
				p.SortOrder,
			),
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
			WorkspaceId: uuidPtrToStr(p.WorkspaceID),
		}
	}

	var archiveHours int32
	if agg.Archive != nil {
		archiveHours = int32( //nolint:gosec // int32 safe for domain values
			agg.Archive.ArchiveAfterHours,
		)
	}

	return connect.NewResponse(&todosv1.GetSettingsResponse{
		LabelPresets: protoPresets,
		UrlPatterns:  protoPatterns,
		Archive: &todosv1.ArchiveSettings{
			UserId:            userID,
			ArchiveAfterHours: archiveHours,
		},
		Sections:   protoSections(agg.Sections),
		Policies:   protoPolicies,
		Workspaces: protoWorkspaces(agg.Workspaces),
		UserSettings: &todosv1.UserSettings{
			UserId:            agg.UserSettings.UserID,
			ActiveWorkspaceId: uuidPtrToStr(agg.UserSettings.ActiveWorkspaceID),
			HideShortcutHints: agg.UserSettings.HideShortcutHints,
		},
	}), nil
}

func (h *settingsConnectHandler) UpdateArchiveSettings(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateArchiveSettingsRequest],
) (*connect.Response[todosv1.UpdateArchiveSettingsResponse], error) {
	userID := h.userID(ctx)
	if err := h.app.services.Settings.UpdateArchiveSettings(
		ctx, userID, int(req.Msg.ArchiveAfterHours),
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdateArchiveSettingsResponse{}), nil
}

func (h *settingsConnectHandler) UpdateHideShortcutHints(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateHideShortcutHintsRequest],
) (*connect.Response[todosv1.UpdateHideShortcutHintsResponse], error) {
	userID := h.userID(ctx)
	if err := h.app.services.Settings.UpdateHideShortcutHints(
		ctx, userID, req.Msg.Hide,
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdateHideShortcutHintsResponse{}), nil
}
