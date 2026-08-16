-- +goose Up
-- +goose StatementBegin
-- Response bytes served per (day, app, endpoint), alongside the existing
-- request count. Request counts alone could not show which endpoint was
-- moving the data when the Supabase egress quota was exhausted (issue
-- #1027) — this is the missing dimension.
ALTER TABLE global.usage_daily
ADD COLUMN IF NOT EXISTS bytes BIGINT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.usage_daily
DROP COLUMN IF EXISTS bytes;
-- +goose StatementEnd
