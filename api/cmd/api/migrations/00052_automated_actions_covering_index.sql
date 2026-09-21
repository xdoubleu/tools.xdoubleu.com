-- +goose Up
-- +goose StatementBegin
-- Issue #1714: ListRecent's WHERE fired_at >= $1 ORDER BY fired_at DESC
-- LIMIT $2 already used idx_automated_actions_fired_at (from
-- 00050_automated_actions.sql) as an efficient Index Scan Backward — an
-- EXPLAIN ANALYZE against a synthetic 2M-row table showed sub-millisecond
-- execution even before this change, so the plain index was never the
-- multi-second bottleneck the regression report assumed. This still swaps
-- it for a covering index (INCLUDE columns matching ListRecent's full
-- projection) so the same query becomes an Index Only Scan as the table
-- grows, with no plan-quality downside. It replaces rather than
-- supplements the plain index, since the covering index serves every
-- predicate/ordering the plain one did (a DESC btree scans backward for
-- ASC order just as well), so keeping both would only add write overhead
-- with no read benefit.
DROP INDEX IF EXISTS idx_automated_actions_fired_at;
CREATE INDEX IF NOT EXISTS idx_automated_actions_fired_at_covering
ON global.automated_actions (fired_at DESC)
INCLUDE (id, trigger_source, routine_name, finished_at, outcome, pr_url, error);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_automated_actions_fired_at_covering;
CREATE INDEX IF NOT EXISTS idx_automated_actions_fired_at
ON global.automated_actions (fired_at);
-- +goose StatementEnd
