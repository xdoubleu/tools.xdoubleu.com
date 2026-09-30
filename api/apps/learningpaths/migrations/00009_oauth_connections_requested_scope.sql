-- +goose Up
-- +goose StatementBegin
ALTER TABLE learningpaths.oauth_connections
ADD COLUMN IF NOT EXISTS requested_scope TEXT;

-- Backfill: every existing connection was authorized when task:add was the
-- only scope requested, so it now needs a reconnect.
UPDATE learningpaths.oauth_connections
SET requested_scope = 'task:add'
WHERE requested_scope IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.oauth_connections
DROP COLUMN IF EXISTS requested_scope;
-- +goose StatementEnd
