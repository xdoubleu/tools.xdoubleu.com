-- +goose Up
-- +goose StatementBegin
-- Journeys now come from iRail, whose station ids are 9-digit UIC codes
-- ("008812005"). Saved commutes held the GTFS parent-station id ("S8812005"),
-- so the column meaning changes: rewrite each to the UIC code. A value not
-- shaped like an S-prefixed UIC id is left alone.
UPDATE trains.saved_commutes
SET origin_stop_id = lpad(substr(origin_stop_id, 2), 9, '0')
WHERE origin_stop_id ~ '^S[0-9]{1,9}$';

UPDATE trains.saved_commutes
SET destination_stop_id = lpad(substr(destination_stop_id, 2), 9, '0')
WHERE destination_stop_id ~ '^S[0-9]{1,9}$';

-- The GTFS timetable and its staging copies are no longer imported.
DROP TABLE IF EXISTS
trains.stop_times, trains.trips, trains.routes, trains.calendar_dates,
trains.transfers, trains.stops, trains.feed_info,
trains.stop_times_staging, trains.trips_staging, trains.routes_staging,
trains.calendar_dates_staging, trains.transfers_staging,
trains.stops_staging
CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The dropped GTFS tables and the old station ids are not restored.
SELECT 1;
-- +goose StatementEnd
