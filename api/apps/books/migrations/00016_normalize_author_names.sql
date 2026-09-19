-- +goose Up
-- +goose StatementBegin
-- Issue #1749: some authors were stored "Lastname, Firstname" (Dublin Core
-- dc:creator convention, ingested verbatim from EPUB metadata before this
-- fix). Flip any single-comma entry to "Firstname Lastname" to match every
-- other source. Entries with zero or two-or-more commas (already correct,
-- or a suffix like "King, Martin Luther, Jr") are left untouched.
UPDATE books.books
SET
    authors = (
        SELECT
            array_agg(
                CASE
                    WHEN a ~ '^[^,]+,[^,]+$'
                        THEN
                            trim(split_part(a, ',', 2))
                            || ' '
                            || trim(split_part(a, ',', 1))
                    ELSE a
                END
            )
        FROM unnest(authors) AS a
    )
WHERE EXISTS (
    SELECT 1 FROM unnest(authors) AS a
    WHERE a ~ '^[^,]+,[^,]+$'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op: which authors were "Lastname, Firstname" before the Up is not
-- recoverable.
-- +goose StatementEnd
