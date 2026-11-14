-- +goose Up
-- +goose StatementBegin
-- The job image digest last seen on the served manifest, so a new one resumes the
-- tasks parked on the sandbox. NULL until first seen; nothing to backfill.
ALTER TABLE factory.board_settings ADD COLUMN IF NOT EXISTS image_digest TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.board_settings DROP COLUMN IF EXISTS image_digest;
-- +goose StatementEnd
