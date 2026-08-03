-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS feeds;

CREATE TABLE IF NOT EXISTS feeds.feeds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    url TEXT,
    title TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'rss',
    inbound_token TEXT,
    etag TEXT,
    last_modified TEXT,
    last_fetched_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, url)
);

CREATE INDEX IF NOT EXISTS idx_feeds_user_id ON feeds.feeds (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS feeds_inbound_token_idx ON feeds.feeds (
    inbound_token
) WHERE inbound_token IS NOT NULL;

CREATE OR REPLACE FUNCTION feeds.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

CREATE TRIGGER trg_feeds_updated_at
BEFORE UPDATE ON feeds.feeds
FOR EACH ROW EXECUTE FUNCTION feeds.set_updated_at();

CREATE TABLE IF NOT EXISTS feeds.items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feed_id UUID NOT NULL REFERENCES feeds.feeds (id) ON DELETE CASCADE,
    guid TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    content_html TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ,
    dismissed BOOLEAN NOT NULL DEFAULT FALSE,
    favourite BOOLEAN NOT NULL DEFAULT FALSE,
    ingest_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (feed_id, guid)
);

CREATE INDEX IF NOT EXISTS idx_items_feed_id ON feeds.items (feed_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS feeds.items;
DROP TABLE IF EXISTS feeds.feeds;
DROP FUNCTION IF EXISTS feeds.set_updated_at();
DROP SCHEMA IF EXISTS feeds;
-- +goose StatementEnd
