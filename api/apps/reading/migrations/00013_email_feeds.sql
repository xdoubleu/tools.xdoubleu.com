-- +goose Up
-- +goose StatementBegin
-- Email-relay feeds (issue #595): a feed subscribed via a per-feed inbound
-- email address instead of a polled RSS URL. url is only meaningful for
-- "rss" feeds, so it must become nullable; the existing (user_id, url)
-- unique constraint already allows multiple NULLs per Postgres semantics,
-- so no email feed collides with another.
ALTER TABLE reading.feeds ALTER COLUMN url DROP NOT NULL;
ALTER TABLE reading.feeds ADD COLUMN source_type TEXT NOT NULL DEFAULT 'rss';
ALTER TABLE reading.feeds ADD COLUMN inbound_token TEXT;
CREATE UNIQUE INDEX feeds_inbound_token_idx ON reading.feeds (inbound_token)
WHERE inbound_token IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX reading.feeds_inbound_token_idx;
ALTER TABLE reading.feeds DROP COLUMN inbound_token;
ALTER TABLE reading.feeds DROP COLUMN source_type;
ALTER TABLE reading.feeds ALTER COLUMN url SET NOT NULL;
-- +goose StatementEnd
