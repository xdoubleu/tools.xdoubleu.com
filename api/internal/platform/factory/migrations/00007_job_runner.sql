-- +goose Up
-- +goose StatementBegin
-- The runner that claimed the job. Empty for a job claimed before this, which
-- any runner may still finish.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS runner_id UUID
REFERENCES factory.runners (id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS runner_id;
-- +goose StatementEnd
