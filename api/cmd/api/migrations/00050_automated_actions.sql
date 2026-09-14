-- +goose Up
-- +goose StatementBegin
-- Issue #1441: the self-healing platform's routines execute on Anthropic's
-- scheduled-agent infrastructure, not in-process, so unlike
-- global.job_runs (populated automatically by TrackedJob) nothing here
-- records a run unless the routine itself does — it opens its own row as
-- its first step (OpenAutomatedAction) and closes it as its last
-- (CloseAutomatedAction). finished_at/outcome/pr_url/error stay NULL while
-- a run is still open.
CREATE TABLE IF NOT EXISTS global.automated_actions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    trigger_source TEXT NOT NULL
    CHECK (trigger_source IN ('schedule', 'api', 'manual')),
    routine_name TEXT NOT NULL,
    finished_at TIMESTAMPTZ,
    outcome TEXT CHECK (outcome IN ('succeeded', 'failed', 'no_action_needed')),
    pr_url TEXT,
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_automated_actions_routine_name_fired_at
ON global.automated_actions (routine_name, fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_automated_actions_fired_at
ON global.automated_actions (fired_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.automated_actions;
-- +goose StatementEnd
