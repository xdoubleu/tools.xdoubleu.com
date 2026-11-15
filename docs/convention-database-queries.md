# Convention: never select a wide TEXT column in a list query

- Enforced by: review; `make lint/scalar-subqueries` and `make test/reads` for scalar subqueries
- Issues: #1027

## Rule

**Never put a wide TEXT column in a list query's column list.** Multi-row reads
and `RETURNING` clauses select `<col> IS NOT NULL AND <col> <> ''` as a boolean;
only a dedicated single-row read selects the column. A query whose result is
discarded (e.g. a job that only needs which rows exist) selects ids only.

## Rule: a scalar subquery returns at most one row

`(SELECT col FROM t WHERE ...)` used as a value fails with SQLSTATE 21000 as
soon as two rows match. It must aggregate, end in `LIMIT 1`, or filter on a
unique key of its table; a table that holds one row by design carries
`-- single-row` in the subquery. `make lint/scalar-subqueries` checks this
against the migrations' primary keys and unique indexes. Reviewers also check
that a new link table or relation adds its fan-out rows to the messy seed
(`api/internal/platform/factory/messyseed`), which `make test/reads` reads
through every RPC, MCP tool and job; it fails on any error.

## Why

Every byte returned costs egress on every request. That's also why
`get_usage_stats` reports response bytes, not just request counts.

## Worked examples

- `feeds`' `itemColumns`/`itemListColumns` — `apps/feeds/internal/repositories/items.go`
- `books`' `bookColumns` — `apps/books/internal/repositories/books_scan.go`

## What violating it looked like

Listing `feeds.items.content_html` exhausted the monthly egress quota and took
the site down (#1027); nothing could say which endpoint was responsible.

A board lookup by PR URL returned a row per linked task once one PR linked
several, and every board read failed until `LIMIT 1` was added; the tests only
ever linked one task per PR.

## Related: allowed cross-schema read direction

Raw SQL crosses schemas only downstream, as reads and foreign keys:
`recipes ← mealplans ← shoppinglist` → [ADR-0038](adr-0038-downstream-cross-schema-reads.md).
Grep downstream repositories before changing an upstream schema.
