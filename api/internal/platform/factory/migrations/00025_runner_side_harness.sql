-- +goose Up
-- +goose StatementBegin
-- The runner's Mac chooses the harness and model through its own config, so the
-- repo defaults are gone; no backfill, nothing reads them. A job's harness and
-- model are now set when a runner claims it, so a queued job holds neither.
ALTER TABLE factory.repos
DROP COLUMN IF EXISTS default_harness,
DROP COLUMN IF EXISTS default_model;
ALTER TABLE factory.jobs DROP CONSTRAINT IF EXISTS jobs_harness_check;
ALTER TABLE factory.jobs
ADD CONSTRAINT jobs_harness_check CHECK (
    harness IN ('', 'claude_code', 'opencode')
);
ALTER TABLE factory.jobs ALTER COLUMN harness SET DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs ALTER COLUMN harness DROP DEFAULT;
UPDATE factory.jobs SET harness = 'claude_code'
WHERE harness = '';
ALTER TABLE factory.jobs DROP CONSTRAINT IF EXISTS jobs_harness_check;
ALTER TABLE factory.jobs
ADD CONSTRAINT jobs_harness_check CHECK (
    harness IN ('claude_code', 'opencode')
);
ALTER TABLE factory.repos
ADD COLUMN IF NOT EXISTS default_harness TEXT NOT NULL DEFAULT 'claude_code'
CHECK (
    default_harness IN ('claude_code', 'opencode')
),
ADD COLUMN IF NOT EXISTS default_model TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
