-- Feeds moved to their own app; only databases built from scratch still have
-- these tables, and nothing reads them.

-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS books.feed_items, books.feeds;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op: the tables held no live data
-- +goose StatementEnd
