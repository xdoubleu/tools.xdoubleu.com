-- New dashboard app (issue #737) centralizes public dashboard sharing: the
-- "books" dashboard share is now the merged books+feeds "reading" dashboard,
-- served by the dashboard app instead of books' own (now-deleted)
-- connect_public.go. Existing share tokens keep working — only the
-- dashboard-kind label they're scoped to changes, same value-rename pattern
-- as 00012_rename_reading_app.sql. This is distinct from (and does not
-- touch) global.app_access/global.usage_daily, which still track the
-- literal "books" app name — only global.profile_shares' dashboard-kind
-- values change here.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.profile_shares SET app = 'reading'
WHERE app = 'books';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('reading', 'games'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.profile_shares
DROP CONSTRAINT profile_shares_app_check;
UPDATE global.profile_shares SET app = 'books'
WHERE app = 'reading';
ALTER TABLE global.profile_shares
ADD CONSTRAINT profile_shares_app_check CHECK (app IN ('books', 'games'));
-- +goose StatementEnd
