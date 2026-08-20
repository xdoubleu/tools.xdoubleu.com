-- +goose Up
-- +goose StatementBegin
-- auth_gotrue_legacy (created by 00017_auth_schema.sql's rename) has served
-- its purpose as a rollback fallback for the GoTrue → first-party auth
-- cutover (issue #1039) and is stable in production. IF EXISTS makes this a
-- no-op on every dev/CI database, which never had it.
DROP SCHEMA IF EXISTS auth_gotrue_legacy CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Not reversible — the dropped schema's contents are gone. A true rollback
-- needs a database restore from backup, not a migration Down.
SELECT 1;
-- +goose StatementEnd
