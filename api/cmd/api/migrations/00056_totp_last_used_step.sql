-- +goose Up
-- +goose StatementBegin
-- The last accepted TOTP time step, so a code can't be replayed within its
-- validity window. NULL until the factor's next successful code; no backfill.
ALTER TABLE auth.totp_factors
ADD COLUMN IF NOT EXISTS last_used_step BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.totp_factors
DROP COLUMN IF EXISTS last_used_step;
-- +goose StatementEnd
