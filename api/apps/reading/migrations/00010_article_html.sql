-- Persist the readability-extracted article HTML on the catalog row so RSS
-- items can be read in-app, instead of only using it transiently to build an
-- EPUB (and discarding it entirely when the feed isn't opted into kobo_sync).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE reading.books ADD COLUMN content_html TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reading.books DROP COLUMN content_html;
-- +goose StatementEnd
