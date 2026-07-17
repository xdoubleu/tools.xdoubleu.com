-- The books app was renamed to "reading" (it now also tracks papers,
-- articles, and RSS posts). Rewrite the stored app identifier everywhere the
-- global schema references it. The reading schema itself is renamed in Go
-- (renameLegacyBooksSchema) before that app's migrations run, because goose's
-- version table lives inside the schema.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.app_access SET app_name = 'reading'
WHERE app_name = 'books';
UPDATE global.profile_shares SET app = 'reading'
WHERE app = 'books';
UPDATE global.usage_daily SET app = 'reading'
WHERE app = 'books';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('reading', 'games'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.usage_daily SET app = 'books'
WHERE app = 'reading';
UPDATE global.profile_shares SET app = 'books'
WHERE app = 'reading';
UPDATE global.app_access SET app_name = 'books'
WHERE app_name = 'reading';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('books', 'games'));
-- +goose StatementEnd
