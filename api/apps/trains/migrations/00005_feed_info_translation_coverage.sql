-- +goose Up
-- +goose StatementBegin
-- A feed with no translations.txt, one whose rows match no stop, and a
-- genuinely monolingual one all render the same way in the station pickers
-- — three identical names — so nothing distinguished them without reading
-- the importer's source. These counts record what each import made of
-- translations.txt and are served by GetFeedInfo (issue #1459).
ALTER TABLE trains.feed_info
ADD COLUMN translated_stops_nl INTEGER NOT NULL DEFAULT 0,
ADD COLUMN translated_stops_fr INTEGER NOT NULL DEFAULT 0,
ADD COLUMN translated_stops_en INTEGER NOT NULL DEFAULT 0,
ADD COLUMN translation_rows INTEGER NOT NULL DEFAULT 0,
ADD COLUMN translation_rows_unmatched INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trains.feed_info
DROP COLUMN translated_stops_nl,
DROP COLUMN translated_stops_fr,
DROP COLUMN translated_stops_en,
DROP COLUMN translation_rows,
DROP COLUMN translation_rows_unmatched;
-- +goose StatementEnd
