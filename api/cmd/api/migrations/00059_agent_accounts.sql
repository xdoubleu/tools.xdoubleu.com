-- +goose Up
-- +goose StatementBegin
-- Agent accounts (adr-0039): one user plus one confidential client_credentials
-- client per account. No existing role changes meaning, so no backfill.
ALTER TABLE global.app_users DROP CONSTRAINT IF EXISTS app_users_role_check;
ALTER TABLE global.app_users ADD CONSTRAINT app_users_role_check
CHECK (role IN ('admin', 'user', 'service', 'agent'));

-- Kept after revocation for history; the client row is deleted on revoke, which
-- cascades to its tokens.
CREATE TABLE IF NOT EXISTS global.agent_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    client_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM auth.oauth2_clients
WHERE id IN (SELECT client_id FROM global.agent_accounts);
DELETE FROM global.app_users
WHERE role = 'agent';
DELETE FROM auth.users
WHERE id IN (SELECT user_id FROM global.agent_accounts);
DROP TABLE IF EXISTS global.agent_accounts;
ALTER TABLE global.app_users DROP CONSTRAINT IF EXISTS app_users_role_check;
ALTER TABLE global.app_users ADD CONSTRAINT app_users_role_check
CHECK (role IN ('admin', 'user', 'service'));
-- +goose StatementEnd
