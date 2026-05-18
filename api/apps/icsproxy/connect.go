package icsproxy

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	icsproxyv1 "tools.xdoubleu.com/gen/icsproxy/v1"
	"tools.xdoubleu.com/gen/icsproxy/v1/icsproxyv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type icsProxyConnectHandler struct{ app *ICSProxy }

var _ icsproxyv1connect.ICSProxyServiceHandler = (*icsProxyConnectHandler)(nil)

func (h *icsProxyConnectHandler) currentUserID(ctx context.Context) string {
	u := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if u == nil {
		return ""
	}
	return u.ID
}

// ListConfigs returns all filter configs for the current user.
func (h *icsProxyConnectHandler) ListConfigs(
	ctx context.Context,
	_ *connect.Request[icsproxyv1.ListConfigsRequest],
) (*connect.Response[icsproxyv1.ListConfigsResponse], error) {
	userID := h.currentUserID(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	configs, _ := h.app.services.Calendar.ListConfigs(ctx, userID)

	protoConfigs := make([]*icsproxyv1.FilterConfig, len(configs))
	for i, cfg := range configs {
		protoConfigs[i] = protoFilterConfig(cfg)
	}

	return connect.NewResponse(&icsproxyv1.ListConfigsResponse{
		Configs: protoConfigs,
	}), nil
}

// PreviewEvents fetches and extracts events from a source URL without saving.
func (h *icsProxyConnectHandler) PreviewEvents(
	ctx context.Context,
	req *connect.Request[icsproxyv1.PreviewEventsRequest],
) (*connect.Response[icsproxyv1.PreviewEventsResponse], error) {
	if req.Msg.SourceUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	data, err := h.app.services.Calendar.FetchICS(ctx, req.Msg.SourceUrl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	events, err := h.app.services.Calendar.ExtractEvents(ctx, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoEvents := make([]*icsproxyv1.EventInfo, len(events))
	for i, e := range events {
		protoEvents[i] = protoEventInfo(e)
	}

	return connect.NewResponse(&icsproxyv1.PreviewEventsResponse{
		Events: protoEvents,
	}), nil
}

// GetConfig returns a config and its current events.
func (h *icsProxyConnectHandler) GetConfig(
	ctx context.Context,
	req *connect.Request[icsproxyv1.GetConfigRequest],
) (*connect.Response[icsproxyv1.GetConfigResponse], error) {
	userID := h.currentUserID(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	cfg, ok := h.app.services.Calendar.LoadConfig(ctx, req.Msg.Token)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	if cfg.UserID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	data, err := h.app.services.Calendar.FetchICS(ctx, cfg.SourceURL)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	events, err := h.app.services.Calendar.ExtractEvents(ctx, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoEvents := make([]*icsproxyv1.EventInfo, len(events))
	for i, e := range events {
		protoEvents[i] = protoEventInfo(e)
	}

	return connect.NewResponse(&icsproxyv1.GetConfigResponse{
		Config: protoFilterConfig(cfg),
		Events: protoEvents,
	}), nil
}

// SaveConfig creates or updates a filter config.
func (h *icsProxyConnectHandler) SaveConfig(
	ctx context.Context,
	req *connect.Request[icsproxyv1.SaveConfigRequest],
) (*connect.Response[icsproxyv1.SaveConfigResponse], error) {
	userID := h.currentUserID(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	token := req.Msg.Token
	if token == "" {
		token = uuid.NewString()
	}

	// Convert hide_series slice to map[string]bool
	hideSeries := make(map[string]bool)
	for _, key := range req.Msg.HideSeries {
		hideSeries[key] = true
	}

	cfg := models.FilterConfig{
		Token:         token,
		UserID:        userID,
		SourceURL:     req.Msg.SourceUrl,
		HideEventUIDs: req.Msg.HideEventUids,
		HolidayUIDs:   req.Msg.HolidayUids,
		HideSeries:    hideSeries,
	}

	if err := h.app.services.Calendar.SaveConfig(ctx, cfg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&icsproxyv1.SaveConfigResponse{
		Token: token,
	}), nil
}

// DeleteConfig removes a filter config.
func (h *icsProxyConnectHandler) DeleteConfig(
	ctx context.Context,
	req *connect.Request[icsproxyv1.DeleteConfigRequest],
) (*connect.Response[icsproxyv1.DeleteConfigResponse], error) {
	userID := h.currentUserID(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if err := h.app.services.Calendar.DeleteConfig(
		ctx, req.Msg.Token, userID,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&icsproxyv1.DeleteConfigResponse{}), nil
}

// Proto conversion helpers

func protoFilterConfig(cfg models.FilterConfig) *icsproxyv1.FilterConfig {
	return &icsproxyv1.FilterConfig{
		Token:         cfg.Token,
		UserId:        cfg.UserID,
		SourceUrl:     cfg.SourceURL,
		HideEventUids: cfg.HideEventUIDs,
		HolidayUids:   cfg.HolidayUIDs,
		HideSeries:    mapsToSlice(cfg.HideSeries),
	}
}

func protoEventInfo(e models.EventInfo) *icsproxyv1.EventInfo {
	return &icsproxyv1.EventInfo{
		Uid:             e.UID,
		Summary:         e.Summary,
		StartRaw:        e.StartRaw,
		EndRaw:          e.EndRaw,
		StartNice:       e.StartNice,
		EndNice:         e.EndNice,
		Rrule:           e.RRule,
		SeriesKey:       e.SeriesKey,
		HasRecurrenceId: e.HasRecurrenceID,
	}
}

// mapsToSlice converts a map[string]bool to a sorted slice of keys.
func mapsToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
