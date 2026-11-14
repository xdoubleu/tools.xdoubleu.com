-- +goose Up
-- +goose StatementBegin
-- The subject the OIDC provider knows the person by (adr-0051). NULL until the
-- user is linked, so no backfill is needed; until then they sign in by password.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS oidc_subject TEXT UNIQUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.users DROP COLUMN IF EXISTS oidc_subject;
-- +goose StatementEnd
