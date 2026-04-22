-- +goose Up
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds
ADD COLUMN user_id TEXT NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds DROP COLUMN user_id;
-- +goose StatementEnd
