-- +goose Up
-- +goose StatementBegin
-- TMDB's watch page for Belgium; empty when TMDB lists no providers.
ALTER TABLE movies.titles
ADD COLUMN IF NOT EXISTS watch_link TEXT NOT NULL DEFAULT '';

-- NULL until the title's providers were first fetched; titles stored before
-- this column existed are backfilled on first open.
ALTER TABLE movies.titles
ADD COLUMN IF NOT EXISTS providers_fetched_at TIMESTAMPTZ;

-- Belgian watch providers per title (not per season), replaced on every
-- TMDB fetch of the title.
CREATE TABLE IF NOT EXISTS movies.title_providers (
    title_id UUID NOT NULL REFERENCES movies.titles (id) ON DELETE CASCADE,
    offer_type TEXT NOT NULL CHECK (
        offer_type IN ('flatrate', 'free', 'ads', 'rent', 'buy')
    ),
    provider_id BIGINT NOT NULL,
    provider_name TEXT NOT NULL,
    logo_path TEXT NOT NULL DEFAULT '',
    display_priority INT NOT NULL DEFAULT 0,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (title_id, offer_type, provider_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS movies.title_providers;
ALTER TABLE movies.titles DROP COLUMN IF EXISTS providers_fetched_at;
ALTER TABLE movies.titles DROP COLUMN IF EXISTS watch_link;
-- +goose StatementEnd
