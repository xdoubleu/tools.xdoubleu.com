-- +goose Up
-- +goose StatementBegin
-- Existing rows start empty; polls backfill items their source still lists.
ALTER TABLE feeds.items ADD COLUMN categories TEXT[] NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE feeds.items DROP COLUMN categories;
-- +goose StatementEnd
