-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.families (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per user who has joined a family. A user with no row here is an
-- implicit family-of-one (issue #1349's confirmed "one family per user,
-- maximum" decision) — apps resolving family membership treat that absence
-- as the no-family-yet case rather than an error.
CREATE TABLE IF NOT EXISTS global.family_members (
    user_id TEXT PRIMARY KEY,
    family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_family_members_family
ON global.family_members (family_id);

-- A pending invitation to join a family. Reuses the accepted-invitation
-- pattern global.contacts already established (invite -> accept/decline)
-- rather than a unilateral add. UNIQUE(to_user_id) alone (no status column)
-- is enough: with at most one family per user, an invitee can have at most
-- one pending invite at a time, and accepting/declining always deletes the
-- row rather than marking it resolved.
CREATE TABLE IF NOT EXISTS global.family_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
    from_user_id TEXT NOT NULL,
    to_user_id TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_family_invites_family
ON global.family_invites (family_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.family_invites;
DROP TABLE IF EXISTS global.family_members;
DROP TABLE IF EXISTS global.families;
-- +goose StatementEnd
