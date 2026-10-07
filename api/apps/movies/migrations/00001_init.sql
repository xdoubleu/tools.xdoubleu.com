-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS movies;

-- Shared TMDB catalog; TMDB's terms cap caching at 6 months.
CREATE TABLE IF NOT EXISTS movies.titles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'series')),
    tmdb_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT NOT NULL DEFAULT '',
    release_date DATE,
    poster_path TEXT NOT NULL DEFAULT '',
    overview TEXT NOT NULL DEFAULT '',
    genres TEXT[] NOT NULL DEFAULT '{}',
    runtime INT,
    season_count INT,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (media_type, tmdb_id)
);

CREATE TABLE IF NOT EXISTS movies.user_titles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    title_id UUID NOT NULL REFERENCES movies.titles (id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (
        status IN ('want', 'watching', 'watched', 'dropped')
    ),
    -- A NULL element is a watch with an unknown date.
    watched_at TIMESTAMPTZ[] NOT NULL DEFAULT '{}',
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, title_id)
);
CREATE INDEX IF NOT EXISTS idx_user_titles_user_added
ON movies.user_titles (
    user_id, added_at DESC
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS movies CASCADE;
-- +goose StatementEnd
