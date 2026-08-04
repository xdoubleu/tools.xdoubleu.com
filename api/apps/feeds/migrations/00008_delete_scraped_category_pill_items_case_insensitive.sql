-- Issue #835 follow-up: 00007's DELETE matched titles case-sensitively
-- against the exact strings as paraphrased in the bug report, not the
-- actual stored casing (e.g. the site may render "Frontier Red Team"
-- title-cased) — an exact-case mismatch means 00007 silently deleted zero
-- rows, and the items were confirmed still present in prod. Retry the same
-- cleanup case-insensitively.

-- +goose Up
-- +goose StatementBegin
DELETE FROM feeds.items i USING feeds.feeds f
WHERE
    i.feed_id = f.id
    AND f.source_type = 'scrape'
    AND lower(btrim(i.title)) IN (
        'frontier red team', 'societal impacts', 'interpretability',
        'economic research'
    );
-- +goose StatementEnd

-- +goose Down
-- Not reversible: deleted items are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
