-- +goose Up
-- +goose StatementBegin
-- 'web-client' marks browser-supplied entries relayed by web's public /logs.
-- A new allowed value only, so no backfill.
ALTER TABLE global.log_entries
DROP CONSTRAINT IF EXISTS log_entries_source_check;
ALTER TABLE global.log_entries
ADD CONSTRAINT log_entries_source_check
CHECK (source IN ('api', 'web', 'web-client'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.log_entries
WHERE source = 'web-client';
ALTER TABLE global.log_entries
DROP CONSTRAINT IF EXISTS log_entries_source_check;
ALTER TABLE global.log_entries
ADD CONSTRAINT log_entries_source_check
CHECK (source IN ('api', 'web'));
-- +goose StatementEnd
