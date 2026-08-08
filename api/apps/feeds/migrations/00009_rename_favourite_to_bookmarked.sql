-- +goose Up
-- +goose StatementBegin
ALTER TABLE feeds.items RENAME COLUMN favourite TO bookmarked;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE feeds.items RENAME COLUMN bookmarked TO favourite;
-- +goose StatementEnd
