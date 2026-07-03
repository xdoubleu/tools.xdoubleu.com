-- The games app registers after books, so by the time this migration runs
-- both apps have adopted or copied everything they need from the former
-- backlog schema. Only the shared leftovers remain; the final DROP SCHEMA is
-- deliberately not CASCADE so it fails loudly if anything unexpected is
-- still inside.

-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS backlog.progress;
DROP TABLE IF EXISTS backlog.user_integrations;
DROP TABLE IF EXISTS backlog.goose_db_version;
DROP FUNCTION IF EXISTS backlog.set_updated_at();
DROP SCHEMA IF EXISTS backlog;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: restore from a database backup instead.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
