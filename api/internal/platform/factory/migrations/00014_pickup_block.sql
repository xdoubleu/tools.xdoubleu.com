-- +goose Up
-- +goose StatementBegin
-- Why a Ready task the factory can't run was passed over: non-empty while it
-- stays in Ready instead of being parked in Needs you, so the column isn't
-- polluted by infra the owner can't act on. A new column, so no backfill.
ALTER TABLE factory.tasks
ADD COLUMN IF NOT EXISTS pickup_blocked TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.tasks
DROP COLUMN IF EXISTS pickup_blocked;
-- +goose StatementEnd
