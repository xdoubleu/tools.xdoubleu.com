-- +goose Up
-- +goose StatementBegin
-- The daily static import used to TRUNCATE + COPY directly into the live
-- tables inside one transaction, held open for the whole ~117s import.
-- TRUNCATE takes an ACCESS EXCLUSIVE lock, which (unlike UPDATE/DELETE)
-- conflicts with even a plain SELECT, so every concurrent read against
-- stops/trips/stop_times/etc. blocked for the entire import window
-- (issue #1718) -- most visibly through StationsService.SearchStations,
-- which queries trains.stops on every request.
--
-- These _staging tables mirror the live ones exactly (same columns, PK,
-- indexes) so FeedRepository.ImportFeed can TRUNCATE + COPY into them
-- instead -- nothing else ever queries a _staging table, so holding it
-- locked for the full import costs nothing -- then swap each staging
-- table into its live counterpart's name via three metadata-only renames,
-- an operation that needs the ACCESS EXCLUSIVE lock on the live table for
-- only milliseconds regardless of the import's own duration.
CREATE TABLE trains.stops_staging (
    stop_id TEXT PRIMARY KEY,
    parent_station TEXT,
    name_nl TEXT NOT NULL,
    name_fr TEXT NOT NULL,
    name_en TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    location_type INTEGER NOT NULL DEFAULT 0,
    platform_code TEXT,
    uic TEXT,
    lat DOUBLE PRECISION,
    lon DOUBLE PRECISION
);
CREATE INDEX ON trains.stops_staging (parent_station);
CREATE INDEX ON trains.stops_staging (uic);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.routes_staging (
    route_id TEXT PRIMARY KEY,
    short_name TEXT,
    long_name TEXT,
    route_type INTEGER NOT NULL DEFAULT 2
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.trips_staging (
    trip_id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    trip_short_name TEXT,
    trip_headsign TEXT,
    direction_id INTEGER
);
CREATE INDEX ON trains.trips_staging (service_id);
CREATE INDEX ON trains.trips_staging (trip_short_name);
CREATE INDEX ON trains.trips_staging (route_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.stop_times_staging (
    trip_id TEXT NOT NULL,
    stop_sequence INTEGER NOT NULL,
    stop_id TEXT NOT NULL,
    arrival_seconds INTEGER NOT NULL,
    departure_seconds INTEGER NOT NULL,
    pickup_type INTEGER NOT NULL DEFAULT 0,
    drop_off_type INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (trip_id, stop_sequence)
);
CREATE INDEX ON trains.stop_times_staging (stop_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.calendar_dates_staging (
    service_id TEXT NOT NULL,
    date DATE NOT NULL,
    exception_type INTEGER NOT NULL,
    PRIMARY KEY (service_id, date)
);
CREATE INDEX ON trains.calendar_dates_staging (date);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.transfers_staging (
    from_stop_id TEXT NOT NULL,
    to_stop_id TEXT NOT NULL,
    transfer_type INTEGER NOT NULL DEFAULT 0,
    min_transfer_time INTEGER,
    PRIMARY KEY (from_stop_id, to_stop_id)
);
CREATE INDEX ON trains.transfers_staging (to_stop_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trains.transfers_staging;
DROP TABLE IF EXISTS trains.calendar_dates_staging;
DROP TABLE IF EXISTS trains.stop_times_staging;
DROP TABLE IF EXISTS trains.trips_staging;
DROP TABLE IF EXISTS trains.routes_staging;
DROP TABLE IF EXISTS trains.stops_staging;
-- +goose StatementEnd
