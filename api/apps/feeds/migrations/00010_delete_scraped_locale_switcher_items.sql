-- Issue #1748: before the locale-alternate check landed in scrape.go's
-- discoverPostLinks heuristic, Uber's engineering blog language switcher
-- (plain <a> tags outside any nav container, e.g. "French, Français
-- (France)") cleared minBarePostLinkTextLen and was ingested as standalone
-- items, once per poll under distinct volatile query-string URLs. The
-- heuristic fixes stop new junk but nothing ever re-evaluates already-
-- ingested items, so the bogus rows lingered in prod and kept the bug
-- looking unfixed after four discovery-side fixes. Delete them.
--
-- ponytail: scoped to the exact language-switcher anchor texts rather than
-- a general pattern — the switcher renders "<Language>, <Endonym>
-- (<Region>)", but a blind prefix/pattern cut risks eating a legitimately
-- titled real post. Case-insensitive (00007's exact-case match silently
-- deleted zero rows); scrape-source-only so RSS/email items are untouched.
-- Confirmed against production before writing: the stored rows all read
-- "French, Français (France)" exactly.

-- +goose Up
-- +goose StatementBegin
DELETE FROM feeds.items i USING feeds.feeds f
WHERE
    i.feed_id = f.id
    AND f.source_type = 'scrape'
    AND lower(btrim(i.title)) IN (
        'english, english',
        'french, français (france)',
        'german, deutsch',
        'dutch, nederlands'
    );
-- +goose StatementEnd

-- +goose Down
-- Not reversible: deleted items are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
