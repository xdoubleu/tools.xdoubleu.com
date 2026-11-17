-- +goose Up
-- +goose StatementBegin
-- A repo's own AGENTS.md replaces per-repo free text; nothing to backfill.
ALTER TABLE factory.repos
DROP COLUMN IF EXISTS setup_commands,
DROP COLUMN IF EXISTS agent_instructions;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.repos
ADD COLUMN IF NOT EXISTS setup_commands TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS agent_instructions TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
