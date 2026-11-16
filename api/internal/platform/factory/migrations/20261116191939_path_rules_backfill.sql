-- +goose Up
-- +goose StatementBegin
-- Set once the one-off move of self-checked Testing tasks to Done has run, so
-- a task moved back to Testing later stays there. False means not yet run; no
-- backfill is needed.
ALTER TABLE factory.board_settings
ADD COLUMN IF NOT EXISTS path_rules_backfilled BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.board_settings DROP COLUMN IF EXISTS path_rules_backfilled;
-- +goose StatementEnd
