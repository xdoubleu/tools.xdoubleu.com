-- +goose Up
-- +goose StatementBegin
-- Per-user OAuth connections (issue #1475), separate from
-- global.oauth_connections: that table's PK is provider alone (one
-- admin-authorized connection per provider, shared by every user), while a
-- Todoist connection belongs to whichever user authorized it, so the PK here
-- must include user_id.
CREATE TABLE IF NOT EXISTS learningpaths.oauth_connections (
    user_id TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('todoist')),
    access_token BYTEA NOT NULL,
    refresh_token BYTEA,
    expires_at TIMESTAMPTZ,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, provider)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learningpaths.oauth_connections;
-- +goose StatementEnd
