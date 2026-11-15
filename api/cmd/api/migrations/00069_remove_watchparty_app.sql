-- The watchparty app was removed (0 requests in 30 days). Strip its app
-- identifier from the global tables that reference apps by name, following
-- 00025_remove_todos_app.sql. The app had no schema of its own.

-- +goose Up
-- +goose StatementBegin
DELETE FROM global.app_access
WHERE app_name = 'watchparty';
DELETE FROM global.usage_daily
WHERE app = 'watchparty';
DELETE FROM global.app_opens
WHERE app = 'watchparty';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deleted rows are not recoverable; the down migration is a no-op.
-- +goose StatementEnd
