-- +goose Up
-- +goose StatementBegin
-- Per-run figures the routine workflow measures from the OpenCode transcript;
-- NULL on rows closed without them. 'ci' is a routine started by a failing
-- push-to-main build. New columns and a new allowed value only, so no
-- backfill.
ALTER TABLE global.automated_actions
ADD COLUMN IF NOT EXISTS requests INTEGER,
ADD COLUMN IF NOT EXISTS input_tokens BIGINT,
ADD COLUMN IF NOT EXISTS output_tokens BIGINT,
ADD COLUMN IF NOT EXISTS reasoning_tokens BIGINT,
ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT,
ADD COLUMN IF NOT EXISTS cost_usd DOUBLE PRECISION,
ADD COLUMN IF NOT EXISTS duration_seconds DOUBLE PRECISION,
ADD COLUMN IF NOT EXISTS tool_calls INTEGER,
ADD COLUMN IF NOT EXISTS tool_errors INTEGER,
ADD COLUMN IF NOT EXISTS repeated_tool_calls INTEGER;

ALTER TABLE global.automated_actions
DROP CONSTRAINT IF EXISTS automated_actions_trigger_source_check;
ALTER TABLE global.automated_actions
ADD CONSTRAINT automated_actions_trigger_source_check
CHECK (trigger_source IN ('schedule', 'api', 'manual', 'ci'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.automated_actions
WHERE trigger_source = 'ci';
ALTER TABLE global.automated_actions
DROP CONSTRAINT IF EXISTS automated_actions_trigger_source_check;
ALTER TABLE global.automated_actions
ADD CONSTRAINT automated_actions_trigger_source_check
CHECK (trigger_source IN ('schedule', 'api', 'manual'));

ALTER TABLE global.automated_actions
DROP COLUMN IF EXISTS requests,
DROP COLUMN IF EXISTS input_tokens,
DROP COLUMN IF EXISTS output_tokens,
DROP COLUMN IF EXISTS reasoning_tokens,
DROP COLUMN IF EXISTS cache_read_tokens,
DROP COLUMN IF EXISTS cost_usd,
DROP COLUMN IF EXISTS duration_seconds,
DROP COLUMN IF EXISTS tool_calls,
DROP COLUMN IF EXISTS tool_errors,
DROP COLUMN IF EXISTS repeated_tool_calls;
-- +goose StatementEnd
