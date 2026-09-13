-- +goose Up
-- +goose StatementBegin
-- display_name is the single canonical passenger-facing station label,
-- built at GTFS ingest time from genuinely-known translations.txt rows only
-- (never a possibly-abbreviated raw stop_name fallback). The backfill below
-- is a best-effort stand-in until the next scheduled static import
-- (TRUNCATE + COPY) overwrites it with the correctly-deduped value
-- (issue #1656).
ALTER TABLE trains.stops ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
UPDATE trains.stops SET display_name = (
    SELECT string_agg(DISTINCT x, ' / ')
    FROM unnest(ARRAY[name_fr, name_nl, name_en]) AS x
    WHERE x <> ''
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trains.stops DROP COLUMN display_name;
-- +goose StatementEnd
