-- +goose Up
-- +goose StatementBegin
-- Catalog seasons of a series; season_number 0 is TMDB's "Specials".
CREATE TABLE IF NOT EXISTS movies.seasons (
    title_id UUID NOT NULL REFERENCES movies.titles (id) ON DELETE CASCADE,
    season_number INT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    air_date DATE,
    episode_count INT NOT NULL DEFAULT 0,
    PRIMARY KEY (title_id, season_number)
);

-- A season is ticked while it has at least one watch; a NULL element is a
-- watch with an unknown date.
CREATE TABLE IF NOT EXISTS movies.user_seasons (
    user_title_id UUID NOT NULL REFERENCES movies.user_titles (
        id
    ) ON DELETE CASCADE,
    season_number INT NOT NULL,
    watched_at TIMESTAMPTZ[] NOT NULL,
    PRIMARY KEY (user_title_id, season_number)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS movies.user_seasons;
DROP TABLE IF EXISTS movies.seasons;
-- +goose StatementEnd
