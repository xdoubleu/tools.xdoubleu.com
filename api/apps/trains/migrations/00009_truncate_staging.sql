-- +goose Up
-- +goose StatementBegin
-- `_staging` held the superseded timetable after every swap (roughly double
-- the trains schema's disk). ImportFeed now truncates it after swapping; this
-- reclaims the copy already there. No backfill: staging is never read.
TRUNCATE
trains.stops_staging,
trains.routes_staging,
trains.trips_staging,
trains.stop_times_staging,
trains.calendar_dates_staging,
trains.transfers_staging;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
