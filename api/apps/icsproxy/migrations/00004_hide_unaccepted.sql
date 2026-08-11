-- +goose Up
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds
ADD COLUMN hide_unaccepted BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds DROP COLUMN hide_unaccepted;
-- +goose StatementEnd
