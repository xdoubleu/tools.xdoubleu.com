# Convention: never select a wide TEXT column in a list query

- Enforced by: nothing but review
- Issues: #1027

## Rule

**Never put a wide TEXT column in a list query's column list.**

Multi-row reads and `RETURNING` clauses select
`<col> IS NOT NULL AND <col> <> ''` as a **boolean**; a dedicated single-row read
is the only query that selects the column itself.

The same applies to any query whose result the caller then throws away — a job
that only needs to know which rows exist should select ids only, not the columns
it's about to discard.

## Why

The deployed database is reached over a **transaction-mode pooler and billed per
byte returned**, so a page of rows carrying a large column is billed egress on
every single request.

This is also why `get_usage_stats` reports response **bytes** and not just
request counts — "which endpoint moves the most data" is the question that
matters here. See `spec-observability-subsystem.md`.

## Worked examples

- `feeds`' `itemColumns`/`itemListColumns` split —
  `apps/feeds/internal/repositories/items.go`
- `books`' `bookColumns` — `apps/books/internal/repositories/books_scan.go`

### `trains` is bound by the same rule

`stop_times` (~0.8M rows) and `calendar_dates` (~1.07M rows) are large, so any
list query the router adds must select only the columns it uses. A careless
full-row read of `stop_times` is the same shape of mistake as
`feeds.items.content_html`.

## What violating it looked like

Getting this wrong on `feeds.items.content_html` **exhausted the whole monthly
egress quota and took the site down** (#1027). Supabase restricted the entire
project, and nothing could say which endpoint caused it, because
`get_usage_stats` counted requests but not bytes at the time.

---

## Related: allowed cross-schema read direction

Downstream apps may **read** an upstream app's schema directly in SQL instead of
going through an internal API. The allowed dependency direction is **acyclic**:

```
recipes ← mealplans ← shoppinglist
```

`mealplans` joins `recipes.recipes`; `shoppinglist`'s export and
item-name-catalog features join both `mealplans.*` and `recipes.*`.

**Reads only, never the reverse direction**, and each app's migrations touch only
its own schema. Grep downstream repositories before changing an upstream schema.
