-- +goose Up
-- +goose StatementBegin
-- StorageScanJob now deletes confirmed orphans past a grace period instead
-- of only reporting them (issue #1328) — these track what that same scan
-- actually removed, a subset of orphan_size_bytes/orphan_count which still
-- count every orphan seen regardless of age or delete outcome.
ALTER TABLE global.storage_snapshots
ADD COLUMN deleted_orphan_size_bytes BIGINT NOT NULL DEFAULT 0,
ADD COLUMN deleted_orphan_count BIGINT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.storage_snapshots
DROP COLUMN deleted_orphan_size_bytes,
DROP COLUMN deleted_orphan_count;
-- +goose StatementEnd
