-- +goose Up
-- +goose StatementBegin
-- An OAuth client an admin delegated to an agent account on the consent page
-- (adr-0048): its tokens act as the agent. A new table, so no backfill.
CREATE TABLE IF NOT EXISTS global.agent_account_clients (
    client_id TEXT PRIMARY KEY REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES global.agent_accounts (id),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.agent_account_clients;
-- +goose StatementEnd
