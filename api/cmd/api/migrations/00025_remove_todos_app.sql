-- The todos app was removed entirely (unused: no open tasks and no activity
-- since May 2026). Strip the "todos" app identifier from every global table
-- that references app names by string, following the 00008_rename_books_app.sql
-- precedent. The todos schema and its own migrations are dropped by the app's
-- own removal, not here — this migration only cleans up cross-app references.

-- +goose Up
-- +goose StatementBegin
DELETE FROM global.app_access
WHERE app_name = 'todos';
DELETE FROM global.usage_daily
WHERE app = 'todos';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deleted rows (which users had todos access, historical usage stats) are not
-- recoverable; the down migration is a no-op.
-- +goose StatementEnd
