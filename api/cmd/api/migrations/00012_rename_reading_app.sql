-- The "reading" app is renamed back to "books" (issue #736) now that
-- arxiv/feeds/category have all been split or removed and it's back to
-- being just a books library. Rewrite the stored app identifier everywhere
-- the global schema references it. The app's own schema is renamed in Go
-- (renameLegacyReadingSchema) before that app's migrations run, because
-- goose's version table lives inside the schema.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.app_access SET app_name = 'books'
WHERE app_name = 'reading';
UPDATE global.profile_shares SET app = 'books'
WHERE app = 'reading';
UPDATE global.usage_daily SET app = 'books'
WHERE app = 'reading';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('books', 'games'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.usage_daily SET app = 'reading'
WHERE app = 'books';
UPDATE global.profile_shares SET app = 'reading'
WHERE app = 'books';
UPDATE global.app_access SET app_name = 'reading'
WHERE app_name = 'books';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('reading', 'games'));
-- +goose StatementEnd
