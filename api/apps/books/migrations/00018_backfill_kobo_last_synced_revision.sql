-- +goose Up
-- +goose StatementBegin
-- Backfill: 00016 added kobo_last_synced_revision as NULL for every existing
-- row, which the sync handler (koboLibrarySyncHandler) reads as "never
-- synced" and answers with NewEntitlement. For a book that was already
-- kobo-sync-enabled before this column existed, the device already has an
-- entry for it — sending NewEntitlement on its next revision bump (a KEPUB
-- regeneration) duplicates it instead of replacing it, reproducing issue
-- #1734 rather than fixing it. Stamp the column with each such book's
-- current revision (the same "<book_id>-v<converter_version>" shape
-- koboRevisionID computes) so the handler treats it as already synced and
-- correctly emits ChangedEntitlement the next time the revision changes.
UPDATE books.user_books ub
SET kobo_last_synced_revision = ub.book_id::text || '-v' || bf.converter_version
FROM books.book_files AS bf
WHERE
    bf.book_id = ub.book_id
    AND bf.user_id = ub.user_id
    AND bf.status = 'ready'
    AND bf.format = CASE
        WHEN 'kobo-format-pdf' = ANY(ub.tags) THEN 'pdf'
        ELSE 'kepub'
    END
    AND ub.kobo_sync_enabled_at IS NOT NULL
    AND ub.kobo_last_synced_revision IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op: data backfill, nothing structural to revert
-- +goose StatementEnd
