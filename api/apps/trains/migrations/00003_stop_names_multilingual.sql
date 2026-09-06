-- +goose Up
-- +goose StatementBegin
-- Station names were French-only; nl/fr/en columns replace the single
-- name column. The daily static import (TRUNCATE + COPY) repopulates all
-- three from the feed's primary stop_name, falling back to it for any
-- language translations.txt doesn't cover (issue #1450).
ALTER TABLE trains.stops
ADD COLUMN name_nl TEXT,
ADD COLUMN name_fr TEXT,
ADD COLUMN name_en TEXT;
UPDATE trains.stops SET name_nl = name, name_fr = name, name_en = name;
ALTER TABLE trains.stops
ALTER COLUMN name_nl SET NOT NULL,
ALTER COLUMN name_fr SET NOT NULL,
ALTER COLUMN name_en SET NOT NULL,
DROP COLUMN name;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trains.stops
ADD COLUMN name TEXT;
UPDATE trains.stops SET name = name_fr;
ALTER TABLE trains.stops
ALTER COLUMN name SET NOT NULL,
DROP COLUMN name_nl,
DROP COLUMN name_fr,
DROP COLUMN name_en;
-- +goose StatementEnd
