-- +goose Up
-- +goose StatementBegin
-- The Kobo firmware keys entitlements on the Entitlement Id and ignores
-- RevisionId changes (calibre-web#3133 packet captures), so the sync handler
-- keeps RevisionId stable at the bare book UUID and detects a regenerated
-- book by its ConverterVersion instead. 00016/00018 stored the last synced
-- revision as "<book_id>-v<converter_version>" TEXT; that value is now
-- reduced to the converter version itself. NULL (never synced) stays NULL.
ALTER TABLE books.user_books
RENAME COLUMN kobo_last_synced_revision TO kobo_last_synced_converter_version;

ALTER TABLE books.user_books
ALTER COLUMN kobo_last_synced_converter_version TYPE SMALLINT
USING (
    regexp_replace(kobo_last_synced_converter_version, '^.*-v', '')
)::SMALLINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.user_books
ALTER COLUMN kobo_last_synced_converter_version TYPE TEXT
USING CASE
    WHEN kobo_last_synced_converter_version IS NULL THEN NULL
    ELSE
        '00000000-0000-0000-0000-000000000000-v'
        || kobo_last_synced_converter_version::TEXT
END;

ALTER TABLE books.user_books
RENAME COLUMN kobo_last_synced_converter_version TO kobo_last_synced_revision;
-- +goose StatementEnd
