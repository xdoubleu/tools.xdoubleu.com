-- +goose Up
-- +goose StatementBegin
-- How many jobs the runner last said it runs at once, so auto pickup can tell
-- whether the online runners have a free slot. Default 1 until it claims.
ALTER TABLE factory.runners
ADD COLUMN IF NOT EXISTS max_jobs INT NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.runners DROP COLUMN IF EXISTS max_jobs;
-- +goose StatementEnd
