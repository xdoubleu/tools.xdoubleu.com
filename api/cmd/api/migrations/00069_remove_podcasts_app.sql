-- The podcasts app was folded into taste: favourite shows are managed on the
-- taste page and its podcasts schema stays as it is. Strip the "podcasts" app
-- identifier from the global tables that reference app names by string. No
-- backfill: taste access governs the shows from now on, and users who had
-- podcasts but not taste keep no access to the favourites.

-- +goose Up
-- +goose StatementBegin
DELETE FROM global.app_access
WHERE app_name = 'podcasts';
DELETE FROM global.usage_daily
WHERE app = 'podcasts';
DELETE FROM global.app_opens
WHERE app = 'podcasts';
UPDATE global.app_users
SET tab_pins = array_remove(tab_pins, 'podcasts')
WHERE 'podcasts' = ANY(tab_pins);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deleted rows (who had podcasts access, usage stats, pins) are not
-- recoverable; the down migration is a no-op.
-- +goose StatementEnd
