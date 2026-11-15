-- +goose Up
-- +goose StatementBegin
-- Per-repo monitoring signals. The defaults reproduce the red-PR scope that was
-- hard-coded before (Renovate's label, "claude/" branches), so existing repos
-- need no backfill; an empty sentry_project means the repo has none.
ALTER TABLE factory.repos
ADD COLUMN IF NOT EXISTS sentry_project TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS red_pr_label TEXT NOT NULL DEFAULT 'dependencies',
ADD COLUMN IF NOT EXISTS red_pr_branch_prefix TEXT NOT NULL DEFAULT 'claude/';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.repos
DROP COLUMN IF EXISTS sentry_project,
DROP COLUMN IF EXISTS red_pr_label,
DROP COLUMN IF EXISTS red_pr_branch_prefix;
-- +goose StatementEnd
