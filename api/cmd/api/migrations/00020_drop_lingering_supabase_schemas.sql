-- +goose Up
-- +goose StatementBegin
-- storage/realtime/vault/public/extensions/graphql_public/graphql were
-- carried over from the original Supabase pg_dump restore (#1029) but were
-- never used by this app's own code — Supabase-managed scaffolding, not
-- app data. gen_random_uuid(), used throughout every app's migrations, is a
-- PostgreSQL 13+ core builtin and does not depend on the extensions schema.
-- IF EXISTS makes this a no-op on every dev/CI database, which never had
-- these schemas.
DROP SCHEMA IF EXISTS storage CASCADE;
DROP SCHEMA IF EXISTS realtime CASCADE;
DROP SCHEMA IF EXISTS vault CASCADE;
DROP SCHEMA IF EXISTS public CASCADE;
DROP SCHEMA IF EXISTS graphql_public CASCADE;
DROP SCHEMA IF EXISTS graphql CASCADE;
DROP SCHEMA IF EXISTS extensions CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Not reversible — the dropped schemas' contents are gone. A true rollback
-- needs a database restore from backup, not a migration Down.
SELECT 1;
-- +goose StatementEnd
