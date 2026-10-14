-- +goose Up
-- +goose StatementBegin
-- When picks were last attempted, failed or not, so the job can back off
-- retrying them. No backfill: NULL means never attempted, which only makes a
-- user with missing picks retry once on the next run.
ALTER TABLE taste.profiles
ADD COLUMN IF NOT EXISTS recommendations_attempted_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE taste.profiles
DROP COLUMN IF EXISTS recommendations_attempted_at;
-- +goose StatementEnd
