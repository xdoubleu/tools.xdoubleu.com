package main

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models"
)

// allOAuthProviders lists every provider the admin UI can show a card for,
// including ones with no stored connection yet.
//
//nolint:gochecknoglobals // fixed provider list, not runtime-configurable
var allOAuthProviders = []models.OAuthProvider{
	models.OAuthProviderGithub,
	models.OAuthProviderSentry,
	models.OAuthProviderDigitalOcean,
}

func (h *obsConnectHandler) ListOAuthConnections(
	ctx context.Context,
	_ *connect.Request[observabilityv1.ListOAuthConnectionsRequest],
) (*connect.Response[observabilityv1.ListOAuthConnectionsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	connections, err := h.app.oauthConnRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	byProvider := make(
		map[models.OAuthProvider]models.OAuthConnection,
		len(connections),
	)
	for _, c := range connections {
		byProvider[c.Provider] = c
	}

	statuses := make([]*observabilityv1.OAuthConnectionStatus, len(allOAuthProviders))
	for i, provider := range allOAuthProviders {
		conn, ok := byProvider[provider]
		if !ok {
			statuses[i] = &observabilityv1.OAuthConnectionStatus{
				Provider:  string(provider),
				Connected: false,
			}
			continue
		}
		statuses[i] = &observabilityv1.OAuthConnectionStatus{
			Provider:    string(provider),
			Connected:   true,
			ConnectedBy: h.resolveConnectedBy(ctx, conn.ConnectedBy),
			ConnectedAt: conn.ConnectedAt.Format(time.RFC3339),
			ExpiresAt:   formatExpiresAt(conn.ExpiresAt),
		}
	}

	return connect.NewResponse(&observabilityv1.ListOAuthConnectionsResponse{
		Connections: statuses,
	}), nil
}

// resolveConnectedBy maps a stored user ID to their email, falling back to
// the raw ID if the user can no longer be found.
func (h *obsConnectHandler) resolveConnectedBy(
	ctx context.Context,
	userID string,
) string {
	user, err := h.app.appUsersRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return userID
	}
	return user.Email
}

func formatExpiresAt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func (h *obsConnectHandler) DisconnectOAuthConnection(
	ctx context.Context,
	req *connect.Request[observabilityv1.DisconnectOAuthConnectionRequest],
) (*connect.Response[observabilityv1.DisconnectOAuthConnectionResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	provider := models.OAuthProvider(req.Msg.GetProvider())
	if err := h.app.oauthConnRepo.Delete(ctx, provider); err != nil {
		h.app.logger.WarnContext(ctx, "failed to delete oauth connection",
			slog.String("provider", string(provider)), slog.Any("error", err))
		return nil, err
	}

	return connect.NewResponse(
		&observabilityv1.DisconnectOAuthConnectionResponse{},
	), nil
}
