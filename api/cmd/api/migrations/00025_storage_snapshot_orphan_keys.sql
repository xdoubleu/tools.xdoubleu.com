-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.storage_snapshots
ADD COLUMN orphan_keys JSONB NOT NULL DEFAULT '[]'::JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.storage_snapshots
DROP COLUMN orphan_keys;
-- +goose StatementEnd
