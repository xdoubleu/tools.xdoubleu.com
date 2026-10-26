# ADR-0036: Cross-schema SQL runs downstream only, as reads and foreign keys

- Status: Accepted
- Issues: #2618 (wave 2 of #2596)
- Affects: `api/apps/mealplans`, `api/apps/shoppinglist`, `api/scripts/check_cross_schema.sh`

## Context

Each app owns a Postgres schema. mealplans and shoppinglist nevertheless read
the recipes and mealplans schemas in raw SQL and hold foreign keys into them
(`mealplans/internal/repositories/plans.go`, `shoppinglist/internal/repositories/`,
`shoppinglist/migrations/00001`, `00002`). One line in `api/AGENTS.md` was the
only record, and nothing stopped a new app from copying the pattern in either
direction.

## Decision

Raw SQL may name another app's schema only along

```
recipes ← mealplans ← shoppinglist
```

- A downstream app may `SELECT`/`JOIN` an upstream table and `REFERENCES` it.
  Upstream never names downstream; apps off this chain never name each other's
  schemas.
- Everything else crosses through exported app methods
  ([ADR-0007](adr-0007-dashboard-app-owns-public-sharing.md)): dashboard,
  taste and learningpaths read books, feeds and the rest that way, and
  learningpaths deliberately has no foreign key.
- Writes into another app's schema are limited to two exceptions:
  - shoppinglist's `SetItemExcluded` sets
    `mealplans.plan_meals.exclude_from_shopping_list`.
  - `mealplans/migrations/00001` adopts the plan tables that began in recipes.
- `make lint/cross-schema` enforces this on non-test Go and SQL. Test files are
  exempt: they seed fixtures across schemas. A new edge or write exception
  means editing the script and this ADR together.

## Alternatives considered

- **Exported methods for every cross read.** The shopping export joins
  `plan_meals`, `recipes` and `ingredients` in one query; going through app
  methods would mean round trips or duplicated aggregation for no ownership
  gain.
- **Drop the foreign keys.** `custom_items.plan_id` cascades on plan delete;
  without the key shoppinglist would have to clean up itself.
- **One shared schema.** Loses per-app ownership of migrations and tables.

## Consequences

- Renaming or dropping an upstream column means grepping downstream
  repositories first; the check does not see column-level coupling.
- A fourth app that needs recipe data joins the chain deliberately or uses
  exported methods.

## Revisit when

The chain grows past three apps, or a downstream write beyond
`exclude_from_shopping_list` is wanted.
