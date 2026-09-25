# ADR-0008: One `family` concept replaces per-app sharing and standalone contacts

- Status: Accepted
- Issues: #1349, #1403
- Affects: `api/internal/family/`, `api/internal/repositories`, `apps/recipes`, `apps/mealplans`, `apps/shoppinglist`, `web/app/family`

## Context

Per-app owner-centric sharing and a standalone `contacts` concept overlapped;
each new app had to pick one and reimplement it.

## Decision

`internal/family` is **the** sharing concept (`contacts` removed in #1403).

- `global.families`/`global.family_members`: at most one family per user, each
  member with a `display_name`. No row means an **implicit family-of-one**,
  materialized by `FamilyRepository.EnsureFamily`.
- `global.family_invites` is **pending-only**; accept/decline deletes the row.
- `InviteByEmail` requires an existing user and emails off the request path (a
  send failure is logged, not returned).
- recipes/mealplans/shoppinglist key data by `family_id` via
  `repositories.FamilyRepository`.

## Alternatives considered

- **Keep `contacts`** — it gated nothing.
- **Multiple families per user** — breaks the single-pending-invite and
  family-of-one invariants.
- **Un-merge data on leave** — explicitly rejected (#1349): the leaver starts a
  fresh solo family.

## Consequences

- Data created in a family stays there; leave-family UI must say so.
- New apps needing sharing take `FamilyRepository` and key by `family_id`.

## Revisit when

A real multi-group need appears (e.g. household plus friends).
