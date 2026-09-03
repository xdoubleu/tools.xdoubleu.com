-- +goose Up
-- +goose StatementBegin
-- A family member's own chosen name, shown to the rest of their family in
-- place of their email. Absorbs the one capability global.contacts still
-- had (an editable display-name alias) now that contacts is removed: since
-- recipes/mealplans/shoppinglist re-keyed onto family_id (issue #1349),
-- contacts gated access to nothing and its invite-by-email flow only
-- duplicated the family one (issue #1403).
-- IF NOT EXISTS: the repositories package's test harness mirrors this schema
-- and CI runs every package against one shared database (see 00040 and
-- job_runs_test.go for the same pattern on the other global tables).
ALTER TABLE global.family_members
ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

DROP TABLE IF EXISTS global.contacts;
-- +goose StatementEnd

-- +goose Down
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

ALTER TABLE global.family_members DROP COLUMN display_name;
-- +goose StatementEnd
