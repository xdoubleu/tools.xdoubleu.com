-- +goose Up
-- +goose StatementBegin
-- The daily import sends the stored ETag/Last-Modified as conditional-GET
-- validators, which describe the feed and not the importer: a change to
-- what an import writes (multilingual stop names, #1450) left the stored
-- rows stale behind a 304 until SNCB happened to publish a new feed.
-- parser_version records which importer produced the stored rows, so a
-- mismatch forces a full re-import on the next run (issue #1453).
-- Existing rows predate multilingual names, hence version 1.
ALTER TABLE trains.feed_info
ADD COLUMN parser_version INTEGER NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trains.feed_info
DROP COLUMN parser_version;
-- +goose StatementEnd
