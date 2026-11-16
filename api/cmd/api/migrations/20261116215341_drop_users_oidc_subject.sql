-- +goose Up
-- +goose StatementBegin
-- Zitadel sign-in was removed before any user was linked, so every
-- oidc_subject is NULL and nothing needs a backfill.
ALTER TABLE auth.users DROP COLUMN IF EXISTS oidc_subject;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS oidc_subject TEXT UNIQUE;
-- +goose StatementEnd
