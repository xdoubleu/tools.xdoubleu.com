-- Issue #835: before the scrape.go fix, discoverPostLinks' minPostLinkTextLen
-- bar (15 chars) was low enough that a scraped page's topic/category filter
-- pills (plain <a> tags with no nested <time>/heading, e.g. Anthropic's
-- research index) were ingested as standalone items rather than being
-- excluded as nav/filter chrome. Delete the specific bogus items reported
-- still lingering in prod after the fix landed.
--
-- ponytail: scoped to the exact reported titles rather than a general
-- length-based cleanup — a blind cutoff risks deleting legitimately
-- short-titled real posts already in the table. Other not-yet-reported
-- bogus items from this class of bug, if any, are left for manual review.

-- +goose Up
-- +goose StatementBegin
DELETE FROM feeds.items i USING feeds.feeds f
WHERE
    i.feed_id = f.id
    AND f.source_type = 'scrape'
    AND btrim(i.title) IN (
        'Frontier red team', 'Societal impacts', 'Interpretability',
        'Economic research'
    );
-- +goose StatementEnd

-- +goose Down
-- Not reversible: deleted items are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
