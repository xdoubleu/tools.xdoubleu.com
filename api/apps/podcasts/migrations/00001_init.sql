-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS podcasts;

CREATE TABLE IF NOT EXISTS podcasts.shows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    itunes_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    author TEXT NOT NULL DEFAULT '',
    artwork_url TEXT NOT NULL DEFAULT '',
    -- Always from the iTunes API, never user input.
    feed_url TEXT NOT NULL,
    apple_url TEXT NOT NULL DEFAULT '',
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, itunes_id)
);
CREATE INDEX IF NOT EXISTS idx_shows_user_added
ON podcasts.shows (
    user_id, added_at DESC
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS podcasts CASCADE;
-- +goose StatementEnd
