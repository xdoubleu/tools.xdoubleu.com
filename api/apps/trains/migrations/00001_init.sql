-- +goose Up
-- +goose StatementBegin
CREATE TABLE trains.stops (
    stop_id TEXT PRIMARY KEY,
    parent_station TEXT,
    name TEXT NOT NULL,
    location_type INTEGER NOT NULL DEFAULT 0,
    platform_code TEXT,
    -- uic is the bare 7-digit UIC code parsed out of stop_id
    -- (gs:nmbssncb:S8814001 / gs:nmbssncb:8814001_3 -> 8814001), the join key
    -- to iRail's zero-padded 9-digit form (a pure left-pad). See issue #1389.
    uic TEXT,
    lat DOUBLE PRECISION,
    lon DOUBLE PRECISION
);
CREATE INDEX ON trains.stops (parent_station);
CREATE INDEX ON trains.stops (uic);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.routes (
    route_id TEXT PRIMARY KEY,
    short_name TEXT,
    long_name TEXT,
    route_type INTEGER NOT NULL DEFAULT 2
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.trips (
    trip_id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    -- trip_short_name is the published train number. trip_id itself is a
    -- seasonal stopping-pattern variant that churns daily and must never
    -- become a long-lived foreign key from user data (issue #1388 slices
    -- 6 & 8) -- group user-facing output by trip_short_name/route.
    trip_short_name TEXT,
    trip_headsign TEXT,
    direction_id INTEGER
);
CREATE INDEX ON trains.trips (service_id);
CREATE INDEX ON trains.trips (trip_short_name);
CREATE INDEX ON trains.trips (route_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE trains.stop_times (
    trip_id TEXT NOT NULL,
    stop_sequence INTEGER NOT NULL,
    stop_id TEXT NOT NULL,
    -- seconds since GTFS "noon minus 12h"; legitimately >= 86400 for
    -- after-midnight service, so this is not a wall-clock time. The importer
    -- rejects values beyond a sane bound (publisher bug: 87:39:00 observed).
    arrival_seconds INTEGER NOT NULL,
    departure_seconds INTEGER NOT NULL,
    -- pickup_type / drop_off_type = 1 marks a non-boarding technical
    -- pass-through (~26.6% of rows); the router (slice 3) must filter these.
    pickup_type INTEGER NOT NULL DEFAULT 0,
    drop_off_type INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (trip_id, stop_sequence)
);
CREATE INDEX ON trains.stop_times (stop_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- calendar.txt is a decoy in this feed: all seven weekday booleans are 0 for
-- every service. The entire operating pattern lives here as exception_type=1
-- (added) rows, ~1.07M of them. Resolve service days from this table ONLY.
CREATE TABLE trains.calendar_dates (
    service_id TEXT NOT NULL,
    date DATE NOT NULL,
    exception_type INTEGER NOT NULL,
    PRIMARY KEY (service_id, date)
);
CREATE INDEX ON trains.calendar_dates (date);
-- +goose StatementEnd

-- +goose StatementBegin
-- Single-row table: feed_version drives the CC BY attribution date and the
-- etag / last_modified columns arm the next run's conditional GET.
CREATE TABLE trains.feed_info (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
    feed_version TEXT NOT NULL,
    feed_start_date DATE,
    feed_end_date DATE,
    feed_lang TEXT,
    etag TEXT,
    last_modified TEXT,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT feed_info_singleton CHECK (singleton)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trains.feed_info;
DROP TABLE IF EXISTS trains.calendar_dates;
DROP TABLE IF EXISTS trains.stop_times;
DROP TABLE IF EXISTS trains.trips;
DROP TABLE IF EXISTS trains.routes;
DROP TABLE IF EXISTS trains.stops;
-- +goose StatementEnd
