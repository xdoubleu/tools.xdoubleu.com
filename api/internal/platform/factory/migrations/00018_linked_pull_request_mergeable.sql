-- +goose Up
-- +goose StatementBegin
-- Whether an armed pull request can merge, as the poller last read it. New
-- columns, empty until the next poll, so no backfill.
ALTER TABLE factory.linked_pull_requests
ADD COLUMN IF NOT EXISTS mergeable TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS unmergeable_since TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.linked_pull_requests
DROP COLUMN IF EXISTS unmergeable_since,
DROP COLUMN IF EXISTS mergeable;
-- +goose StatementEnd
