-- +goose Up
-- +goose StatementBegin
-- New columns with defaults, so no backfill: existing picks start untouched.
ALTER TABLE taste.recommendations
ADD COLUMN IF NOT EXISTS feedback TEXT NOT NULL DEFAULT ''
CHECK (feedback IN ('', 'hit', 'miss', 'not_interested', 'known')),
ADD COLUMN IF NOT EXISTS feedback_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS saved BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS accepted BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS on_my_services BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE taste.recommendations
DROP COLUMN IF EXISTS on_my_services,
DROP COLUMN IF EXISTS accepted,
DROP COLUMN IF EXISTS saved,
DROP COLUMN IF EXISTS feedback_at,
DROP COLUMN IF EXISTS feedback;
-- +goose StatementEnd
