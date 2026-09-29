-- +goose Up
-- +goose StatementBegin
-- New column, so no backfill: no item has been restored yet.
ALTER TABLE feeds.items ADD COLUMN restored_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE feeds.items DROP COLUMN restored_at;
-- +goose StatementEnd
