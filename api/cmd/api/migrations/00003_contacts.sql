-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    contact_user_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, contact_user_id)
);
CREATE INDEX IF NOT EXISTS idx_contacts_owner
ON global.contacts (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_contacts_contact
ON global.contacts (contact_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.contacts;
-- +goose StatementEnd
