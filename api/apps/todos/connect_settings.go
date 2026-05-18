package todos

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *settingsConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	return u.ID
}

func (h *settingsConnectHandler) GetSettings(
	ctx context.Context,
	_ *connect.Request[todosv1.GetSettingsRequest],
) (*connect.Response[todosv1.GetSettingsResponse], error) {
	userID := h.userID(ctx)

	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	wsID := wsCtx.Settings.ActiveWorkspaceID

	presets, err := h.app.services.Settings.GetLabelPresets(ctx, userID, wsID)
	if err != nil {
		return nil, connectErr(err)
	}

	patterns, err := h.app.services.Settings.GetURLPatterns(ctx, userID, wsID)
	if err != nil {
		return nil, connectErr(err)
	}

	archive, err := h.app.services.Settings.GetArchiveSettings(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}

	sections, err := h.app.services.Sections.List(ctx, userID, wsID)
	if err != nil {
		return nil, connectErr(err)
	}

	policies, err := h.app.services.Policies.List(ctx, userID, wsID)
	if err != nil {
		return nil, connectErr(err)
	}

	protoPresets := make([]*todosv1.LabelPreset, len(presets.Labels))
	for i, p := range presets.Labels {
		protoPresets[i] = &todosv1.LabelPreset{Value: p.Value, Color: p.Color}
	}

	protoPatterns := make([]*todosv1.URLPattern, len(patterns))
	for i, p := range patterns {
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

	protoPolicies := make([]*todosv1.Policy, len(policies))
	for i, p := range policies {
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
	if archive != nil {
		archiveHours = int32( //nolint:gosec // int32 safe for domain values
			archive.ArchiveAfterHours,
		)
	}

	return connect.NewResponse(&todosv1.GetSettingsResponse{
		LabelPresets: protoPresets,
		UrlPatterns:  protoPatterns,
		Archive: &todosv1.ArchiveSettings{
			UserId:            userID,
			ArchiveAfterHours: archiveHours,
		},
		Sections:   protoSections(sections),
		Policies:   protoPolicies,
		Workspaces: protoWorkspaces(wsCtx.Workspaces),
		UserSettings: &todosv1.UserSettings{
			UserId:            wsCtx.Settings.UserID,
			ActiveWorkspaceId: uuidPtrToStr(wsCtx.Settings.ActiveWorkspaceID),
			HideShortcutHints: wsCtx.Settings.HideShortcutHints,
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

func (h *settingsConnectHandler) AddLabelPreset(
	ctx context.Context,
	req *connect.Request[todosv1.AddLabelPresetRequest],
) (*connect.Response[todosv1.AddLabelPresetResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	dto := dtos.AddLabelPresetDto{
		Category: req.Msg.Category,
		Value:    req.Msg.Value,
	}
	if addErr := h.app.services.Settings.AddLabelPreset(
		ctx, userID, dto, workspaceID,
	); addErr != nil {
		return nil, connectErr(addErr)
	}
	return connect.NewResponse(&todosv1.AddLabelPresetResponse{}), nil
}

func (h *settingsConnectHandler) RemoveLabelPreset(
	ctx context.Context,
	req *connect.Request[todosv1.RemoveLabelPresetRequest],
) (*connect.Response[todosv1.RemoveLabelPresetResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	if removeErr := h.app.services.Settings.RemoveLabelPreset(
		ctx, userID, req.Msg.Category, req.Msg.Value, workspaceID,
	); removeErr != nil {
		return nil, connectErr(removeErr)
	}
	return connect.NewResponse(&todosv1.RemoveLabelPresetResponse{}), nil
}

func (h *settingsConnectHandler) UpdateLabelColor(
	ctx context.Context,
	req *connect.Request[todosv1.UpdateLabelColorRequest],
) (*connect.Response[todosv1.UpdateLabelColorResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	if updateErr := h.app.services.Settings.UpdateLabelColor(
		ctx, userID, req.Msg.Category, req.Msg.Value, workspaceID,
		req.Msg.Color,
	); updateErr != nil {
		return nil, connectErr(updateErr)
	}
	return connect.NewResponse(&todosv1.UpdateLabelColorResponse{}), nil
}

func (h *settingsConnectHandler) AddURLPattern(
	ctx context.Context,
	req *connect.Request[todosv1.AddURLPatternRequest],
) (*connect.Response[todosv1.AddURLPatternResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	dto := dtos.AddURLPatternDto{
		URLPrefix:    req.Msg.UrlPrefix,
		PlatformName: req.Msg.PlatformName,
		Label:        req.Msg.Label,
		Shortcut:     req.Msg.Shortcut,
	}
	if addErr := h.app.services.Settings.AddURLPattern(
		ctx, userID, dto, workspaceID,
	); addErr != nil {
		return nil, connectErr(addErr)
	}
	return connect.NewResponse(&todosv1.AddURLPatternResponse{}), nil
}

func (h *settingsConnectHandler) RemoveURLPattern(
	ctx context.Context,
	req *connect.Request[todosv1.RemoveURLPatternRequest],
) (*connect.Response[todosv1.RemoveURLPatternResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Settings.RemoveURLPattern(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.RemoveURLPatternResponse{}), nil
}

func (h *settingsConnectHandler) AddSection(
	ctx context.Context,
	req *connect.Request[todosv1.AddSectionRequest],
) (*connect.Response[todosv1.AddSectionResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	dto := dtos.AddSectionDto{Name: req.Msg.Name}
	if _, err = h.app.services.Sections.Create(ctx, userID, dto, workspaceID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.AddSectionResponse{}), nil
}

func (h *settingsConnectHandler) RemoveSection(
	ctx context.Context,
	req *connect.Request[todosv1.RemoveSectionRequest],
) (*connect.Response[todosv1.RemoveSectionResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Sections.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.RemoveSectionResponse{}), nil
}

func (h *settingsConnectHandler) AddPolicy(
	ctx context.Context,
	req *connect.Request[todosv1.AddPolicyRequest],
) (*connect.Response[todosv1.AddPolicyResponse], error) {
	userID := h.userID(ctx)
	wsCtx, err := h.app.loadWorkspaceCtx(ctx, userID)
	if err != nil {
		return nil, connectErr(err)
	}
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if workspaceID == nil {
		workspaceID = wsCtx.Settings.ActiveWorkspaceID
	}
	if _, err = h.app.services.Policies.Create(
		ctx, userID, req.Msg.Text, int(req.Msg.ReappearAfterHours), workspaceID,
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.AddPolicyResponse{}), nil
}

func (h *settingsConnectHandler) UpdatePolicy(
	ctx context.Context,
	req *connect.Request[todosv1.UpdatePolicyRequest],
) (*connect.Response[todosv1.UpdatePolicyResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if _, err = h.app.services.Policies.Update(
		ctx, id, userID, req.Msg.Text, int(req.Msg.ReappearAfterHours),
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.UpdatePolicyResponse{}), nil
}

func (h *settingsConnectHandler) RemovePolicy(
	ctx context.Context,
	req *connect.Request[todosv1.RemovePolicyRequest],
) (*connect.Response[todosv1.RemovePolicyResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Policies.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.RemovePolicyResponse{}), nil
}

func (h *settingsConnectHandler) AddWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.AddWorkspaceRequest],
) (*connect.Response[todosv1.AddWorkspaceResponse], error) {
	userID := h.userID(ctx)
	if _, err := h.app.services.Workspaces.Create(ctx, userID, req.Msg.Name); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.AddWorkspaceResponse{}), nil
}

func (h *settingsConnectHandler) DeleteWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.DeleteWorkspaceRequest],
) (*connect.Response[todosv1.DeleteWorkspaceResponse], error) {
	userID := h.userID(ctx)
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err = h.app.services.Workspaces.Delete(ctx, id, userID); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.DeleteWorkspaceResponse{}), nil
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

func (h *settingsConnectHandler) SetActiveWorkspace(
	ctx context.Context,
	req *connect.Request[todosv1.SetActiveWorkspaceRequest],
) (*connect.Response[todosv1.SetActiveWorkspaceResponse], error) {
	userID := h.userID(ctx)
	var workspaceID *uuid.UUID
	if req.Msg.WorkspaceId != "" {
		if wsID, parseErr := uuid.Parse(req.Msg.WorkspaceId); parseErr == nil {
			workspaceID = &wsID
		}
	}
	if err := h.app.services.Settings.SetActiveWorkspace(
		ctx, userID, workspaceID,
	); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&todosv1.SetActiveWorkspaceResponse{}), nil
}
