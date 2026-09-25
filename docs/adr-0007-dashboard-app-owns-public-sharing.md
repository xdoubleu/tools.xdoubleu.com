# ADR-0007: A schema-less `dashboard` app owns the public dashboards and share tokens

- Status: Accepted
- Issues: #737
- Affects: `api/apps/dashboard/`, `api/apps/games`, `api/apps/books`, `api/apps/feeds`, `web/app/dashboard/`

## Context

Public Games and Reading dashboards and their share tokens were duplicated
across `games` and `books`, and a Reading dashboard spanning books *and* feeds
had no home.

## Decision

`dashboard` owns both dashboards (owner and public views, under
`web/app/dashboard/{games,reading}/`) and the share-token lifecycle.

- It **owns no schema** and reads other apps only through exported methods on
  their structs (`BuildSharedSteam`, `BuildSharedLibrary`, `BuildSummary`, …).
- `PublicGamesDashboardService`/`PublicReadingDashboardService`
  (`connect_public.go`) have **no auth middleware**. Each request carries an
  opaque token (`global.profile_shares`, keyed `(user_id, app)`) resolving to
  the owner. **Public handlers must never read the user-context key.**
- Owners manage tokens and display name via `DashboardService`
  (`cmd/api/connect_dashboard.go`) behind normal `Access`, **not** `dashboard`'s
  own `AppAccess`. `dashboard.Routes()` registers only the public services.

## Alternatives considered

- **Dashboards per app** — duplicated token lifecycle, no home for Reading.
- **Own schema with copied data** — needs invalidation on every upstream write.

## Consequences

Anything `dashboard` needs must be exported on the owning app's struct — a
visible coupling point, never a schema or `internal/` read.

## Revisit when

A third consumer needs the same cross-app reads (argues for a shared read model).
