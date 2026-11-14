-- +goose Up
-- +goose StatementBegin
-- The subject the OIDC provider knows the person by (adr-0051). NULL until the
-- user is linked, so no backfill is needed; until then they sign in by password.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS oidc_subject TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_oidc_subject_key
ON auth.users (oidc_subject) WHERE oidc_subject IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS auth.users_oidc_subject_key;
ALTER TABLE auth.users DROP COLUMN IF EXISTS oidc_subject;
-- +goose StatementEnd
