-- +goose Up
-- +goose StatementBegin
-- Tracks when a refresh token was rotated out, so Store.GetRefreshTokenSession
-- (api/internal/oauth2as/storage.go) can grant a short reuse grace period
-- instead of treating every replay as theft — see issue #1166.
ALTER TABLE auth.oauth2_refresh_tokens ADD COLUMN rotated_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.oauth2_refresh_tokens DROP COLUMN rotated_at;
-- +goose StatementEnd
