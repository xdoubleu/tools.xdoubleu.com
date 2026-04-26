-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_feeds_user_id ON icsproxy.feeds (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS icsproxy.idx_feeds_user_id;
-- +goose StatementEnd
