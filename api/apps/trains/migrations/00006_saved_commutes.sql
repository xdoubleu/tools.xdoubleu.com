-- +goose Up
-- +goose StatementBegin
-- A user's named origin->destination station pairs, surfaced above the
-- /trains pickers so a daily route is one tap away (issue #1396). origin and
-- destination hold the S-prefixed UIC parent-station stop_id from the static
-- feed -- stable UIC codes, never a trip_id or anything derived from a
-- specific day's timetable (issue #1390). Keyed on user_id like every other
-- app here; family re-keying is deferred to issue #1380.
CREATE TABLE trains.saved_commutes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    label TEXT NOT NULL,
    origin_stop_id TEXT NOT NULL,
    destination_stop_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, origin_stop_id, destination_stop_id)
);
CREATE INDEX ON trains.saved_commutes (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trains.saved_commutes;
-- +goose StatementEnd
