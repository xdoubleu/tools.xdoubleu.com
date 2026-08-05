-- +goose Up
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds RENAME COLUMN token_hash TO token;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds RENAME COLUMN token TO token_hash;
-- +goose StatementEnd
