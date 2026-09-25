# Convention: never select a wide TEXT column in a list query

- Enforced by: nothing but review
- Issues: #1027

## Rule

**Never put a wide TEXT column in a list query's column list.** Multi-row reads
and `RETURNING` clauses select `<col> IS NOT NULL AND <col> <> ''` as a boolean;
only a dedicated single-row read selects the column. A query whose result is
discarded (e.g. a job that only needs which rows exist) selects ids only.

## Why

Every byte returned costs egress on every request. That's also why
`get_usage_stats` reports response bytes, not just request counts.

## Worked examples

- `feeds`' `itemColumns`/`itemListColumns` — `apps/feeds/internal/repositories/items.go`
- `books`' `bookColumns` — `apps/books/internal/repositories/books_scan.go`
- `trains`: `stop_times` (~0.8M rows) and `calendar_dates` (~1.07M) — router
  list queries select only the columns they use.

## What violating it looked like

Listing `feeds.items.content_html` exhausted the monthly egress quota and took
the site down (#1027); nothing could say which endpoint was responsible.

## Related: allowed cross-schema read direction

Downstream apps may **read** an upstream schema directly, acyclically:

```
recipes ← mealplans ← shoppinglist
```

Reads only, never the reverse; each app's migrations touch only its own schema.
Grep downstream repositories before changing an upstream schema.
