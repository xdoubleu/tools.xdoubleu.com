-- +goose Up
-- +goose StatementBegin
-- NULL means the runner has not reported an allowlist (an older runner): no
-- restriction is assumed. Nothing to backfill.
ALTER TABLE factory.runners ADD COLUMN repos text[];
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.runners DROP COLUMN IF EXISTS repos;
-- +goose StatementEnd
