-- +goose Up
-- +goose StatementBegin
-- Static confidential OAuth client for Grafana SSO (issue #1469). Unlike the
-- MCP clients (created only through RFC 7591 dynamic registration, always
-- public), Grafana needs a fixed client_id/secret pair it can be configured
-- with ahead of time. The bcrypt hash of the secret is NOT set here — the api
-- writes it into secret_hash on startup from OAUTH_GRAFANA_CLIENT_SECRET
-- (oauth2as.EnsureGrafanaClientSecret), keeping the plaintext secret out of
-- version control. Until that runs the client exists but cannot authenticate.
--
-- The redirect URI points at the planned Grafana base URL (tools.xdoubleu.com/
-- grafana); the Grafana deployment itself lands in issue #1468.
INSERT INTO auth.oauth2_clients (
    id, secret_hash, redirect_uris, grant_types,
    response_types, scopes, public, client_name
) VALUES (
    'grafana',
    NULL,
    ARRAY['https://tools.xdoubleu.com/grafana/login/generic_oauth'],
    ARRAY['authorization_code', 'refresh_token'],
    ARRAY['code'],
    ARRAY['openid', 'profile', 'email', 'offline_access'],
    FALSE,
    'Grafana'
) ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM auth.oauth2_clients
WHERE id = 'grafana';
-- +goose StatementEnd
