package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
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

	res, err := h.oauthConnections(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// oauthConnections builds every provider's admin-facing status. Shared by the
// Connect handler above and the get_oauth_connections MCP tool.
func (h *obsConnectHandler) oauthConnections(
	ctx context.Context,
) (*observabilityv1.ListOAuthConnectionsResponse, error) {
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
		required := strings.Join(h.requiredScopes(provider), " ")

		conn, ok := byProvider[provider]
		if !ok || h.scopeIsStale(&conn) {
			status := &observabilityv1.OAuthConnectionStatus{
				Provider:      string(provider),
				Connected:     false,
				RequiredScope: required,
			}
			if ok {
				// Stale rather than absent: report what it was authorized
				// with, so the mismatch is visible without database access.
				status.RequestedScope = conn.RequestedScope
				status.GrantedScope = conn.GrantedScope
			}
			statuses[i] = status
			continue
		}
		statuses[i] = &observabilityv1.OAuthConnectionStatus{
			Provider:       string(provider),
			Connected:      true,
			ConnectedBy:    h.resolveConnectedBy(ctx, conn.ConnectedBy),
			ConnectedAt:    conn.ConnectedAt.Format(time.RFC3339),
			ExpiresAt:      formatExpiresAt(conn.ExpiresAt),
			Config:         protoProviderConfig(provider, conn.Config),
			RequestedScope: conn.RequestedScope,
			GrantedScope:   conn.GrantedScope,
			RequiredScope:  required,
		}
	}

	return &observabilityv1.ListOAuthConnectionsResponse{
		Connections: statuses,
	}, nil
}

// scopeIsStale reports whether a stored connection was authorized with less
// than what provider's oauth2.Config currently requires — e.g. an admin
// connected before a required scope was added. Such a connection is shown as
// not-connected so the existing "Connect" button in the admin UI is the
// reconnect path, rather than the admin discovering it via a runtime 403.
func (h *obsConnectHandler) scopeIsStale(conn *models.OAuthConnection) bool {
	return oauthconn.ScopesAreStale(conn, h.requiredScopes(conn.Provider))
}

// requiredScopes is what provider's oauth2.Config asks for today, or nil for
// a provider with no OAuth config (which can never be stale).
func (h *obsConnectHandler) requiredScopes(
	provider models.OAuthProvider,
) []string {
	def, ok := oauthProviders[string(provider)]
	if !ok {
		return nil
	}
	return def.conf(h.app).Scopes
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

// githubConfigJSON/sentryConfigJSON/doConfigJSON mirror the private JSON
// shapes each provider client unmarshals from global.oauth_connections.config
// (see internal/{github,sentryapi,digitalocean}/client.go) — kept in sync
// deliberately rather than exported, so the wire shape stays an
// implementation detail of the storage format.
type githubConfigJSON struct {
	Repo string `json:"repo"`
}

type sentryConfigJSON struct {
	Org      string   `json:"org"`
	Projects []string `json:"projects"`
}

type doConfigJSON struct {
	AppID string `json:"app_id"`
}

// protoProviderConfig decodes the stored config JSON into the proto oneof
// for the admin UI. Returns nil (unset) when there's nothing stored yet or
// the provider is unrecognized.
func protoProviderConfig(
	provider models.OAuthProvider, raw json.RawMessage,
) *observabilityv1.ProviderConfig {
	if len(raw) == 0 {
		return nil
	}

	switch provider {
	case models.OAuthProviderGithub:
		var cfg githubConfigJSON
		if json.Unmarshal(raw, &cfg) != nil || cfg.Repo == "" {
			return nil
		}
		return &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Github{
				Github: &observabilityv1.GithubConfig{Repo: cfg.Repo},
			},
		}
	case models.OAuthProviderSentry:
		var cfg sentryConfigJSON
		if json.Unmarshal(raw, &cfg) != nil || cfg.Org == "" {
			return nil
		}
		return &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Sentry{
				Sentry: &observabilityv1.SentryConfig{
					Org: cfg.Org, Projects: cfg.Projects,
				},
			},
		}
	case models.OAuthProviderDigitalOcean:
		var cfg doConfigJSON
		if json.Unmarshal(raw, &cfg) != nil || cfg.AppID == "" {
			return nil
		}
		return &observabilityv1.ProviderConfig{
			Config: &observabilityv1.ProviderConfig_Digitalocean{
				Digitalocean: &observabilityv1.DigitalOceanConfig{AppId: cfg.AppID},
			},
		}
	default:
		return nil
	}
}

// GetProviderOptions lets the admin picker discover what identifiers are
// available for a connected provider, by calling straight through to that
// provider's own client.
func (h *obsConnectHandler) GetProviderOptions(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetProviderOptionsRequest],
) (*connect.Response[observabilityv1.GetProviderOptionsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	var (
		resp *observabilityv1.GetProviderOptionsResponse
		err  error
	)

	switch models.OAuthProvider(req.Msg.GetProvider()) {
	case models.OAuthProviderGithub:
		resp, err = h.githubOptions(ctx)
	case models.OAuthProviderSentry:
		resp, err = h.sentryOptions(ctx, req.Msg.GetSentryOrg())
	case models.OAuthProviderDigitalOcean:
		resp, err = h.digitalOceanOptions(ctx)
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("unknown provider"),
		)
	}
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) githubOptions(
	ctx context.Context,
) (*observabilityv1.GetProviderOptionsResponse, error) {
	repos, err := h.app.githubClient.ListRepos(ctx)
	if err != nil {
		return nil, providerOptionsError(err)
	}

	resp := &observabilityv1.GetProviderOptionsResponse{}
	for _, r := range repos {
		resp.Repos = append(resp.Repos, r.FullName)
	}
	return resp, nil
}

func (h *obsConnectHandler) sentryOptions(
	ctx context.Context, org string,
) (*observabilityv1.GetProviderOptionsResponse, error) {
	resp := &observabilityv1.GetProviderOptionsResponse{}

	if org == "" {
		orgs, err := h.app.sentryClient.ListOrgs(ctx)
		if err != nil {
			return nil, providerOptionsError(err)
		}
		for _, o := range orgs {
			resp.SentryOrgs = append(resp.SentryOrgs, o.Slug)
		}
		return resp, nil
	}

	projects, err := h.app.sentryClient.ListProjects(ctx, org)
	if err != nil {
		return nil, providerOptionsError(err)
	}
	for _, p := range projects {
		resp.SentryProjects = append(resp.SentryProjects, p.Slug)
	}
	return resp, nil
}

func (h *obsConnectHandler) digitalOceanOptions(
	ctx context.Context,
) (*observabilityv1.GetProviderOptionsResponse, error) {
	apps, err := h.app.doClient.ListApps(ctx)
	if err != nil {
		return nil, providerOptionsError(err)
	}

	resp := &observabilityv1.GetProviderOptionsResponse{}
	for _, a := range apps {
		resp.Apps = append(resp.Apps, a.Option())
	}
	return resp, nil
}

// providerOptionsError maps a discovery-call failure to a connect error, so
// "not connected yet" reads as a clear client error instead of a 500.
func providerOptionsError(err error) error {
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if errors.Is(err, models.ErrDecryptFailed) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(
			"this connection can no longer be decrypted and must be reconnected",
		))
	}
	return connect.NewError(connect.CodeInternal, err)
}

// SetProviderConfig stores the admin-picked identifier(s) for a connected
// provider.
func (h *obsConnectHandler) SetProviderConfig(
	ctx context.Context,
	req *connect.Request[observabilityv1.SetProviderConfigRequest],
) (*connect.Response[observabilityv1.SetProviderConfigResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	provider := models.OAuthProvider(req.Msg.GetProvider())

	raw, err := configJSON(provider, req.Msg.GetConfig())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if setErr := h.app.oauthConnRepo.SetConfig(ctx, provider, raw); setErr != nil {
		h.app.logger.WarnContext(ctx, "failed to set oauth connection config",
			slog.String("provider", string(provider)), slog.Any("error", setErr))
		return nil, connect.NewError(connect.CodeInternal, setErr)
	}

	return connect.NewResponse(&observabilityv1.SetProviderConfigResponse{}), nil
}

// configJSON marshals the request's ProviderConfig oneof into the JSON shape
// stored in global.oauth_connections.config, validating it matches provider.
func configJSON(
	provider models.OAuthProvider, cfg *observabilityv1.ProviderConfig,
) ([]byte, error) {
	switch provider {
	case models.OAuthProviderGithub:
		gh := cfg.GetGithub()
		if gh == nil || gh.GetRepo() == "" {
			return nil, errors.New("repo is required")
		}
		return json.Marshal(githubConfigJSON{Repo: gh.GetRepo()})
	case models.OAuthProviderSentry:
		s := cfg.GetSentry()
		if s == nil || s.GetOrg() == "" || len(s.GetProjects()) == 0 {
			return nil, errors.New("org and at least one project are required")
		}
		return json.Marshal(
			sentryConfigJSON{Org: s.GetOrg(), Projects: s.GetProjects()},
		)
	case models.OAuthProviderDigitalOcean:
		do := cfg.GetDigitalocean()
		if do == nil || do.GetAppId() == "" {
			return nil, errors.New("app_id is required")
		}
		return json.Marshal(
			doConfigJSON{AppID: digitalocean.AppIDFromOption(do.GetAppId())},
		)
	default:
		return nil, errors.New("unknown provider")
	}
}
