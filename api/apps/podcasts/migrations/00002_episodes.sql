-- +goose Up
-- +goose StatementBegin
ALTER TABLE podcasts.shows
ADD COLUMN IF NOT EXISTS etag TEXT,
ADD COLUMN IF NOT EXISTS last_modified TEXT,
ADD COLUMN IF NOT EXISTS last_fetched_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS fetch_error TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS podcasts.episodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    show_id UUID NOT NULL REFERENCES podcasts.shows (id) ON DELETE CASCADE,
    guid TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    link TEXT NOT NULL DEFAULT '',
    audio_url TEXT NOT NULL DEFAULT '',
    duration_seconds INT,
    published_at TIMESTAMPTZ,
    UNIQUE (show_id, guid)
);
CREATE INDEX IF NOT EXISTS idx_episodes_show_published
ON podcasts.episodes (
    show_id, published_at DESC
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS podcasts.episodes;
ALTER TABLE podcasts.shows
DROP COLUMN IF EXISTS etag,
DROP COLUMN IF EXISTS last_modified,
DROP COLUMN IF EXISTS last_fetched_at,
DROP COLUMN IF EXISTS fetch_error;
-- +goose StatementEnd
