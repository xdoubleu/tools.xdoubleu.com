# ADR-0007: A schema-less `dashboard` app owns the public dashboards and share tokens

- Status: Accepted
- Issues: #737
- Affects: `api/apps/dashboard/`, `api/apps/games`, `api/apps/books`, `api/apps/feeds`, `web/app/dashboard/`

## Context

The public Games and Reading dashboards, and the share-token lifecycle behind
them, were spread across the `games` and `books` apps — each carrying its own
dashboard-shaped routes and public-sharing code for what is really one feature.

## Decision

**`dashboard` (#737) centralizes** the public Games and Reading (books+feeds)
dashboards — both private/owner and public/shared views — plus the share-token
lifecycle.

It **owns no schema of its own** and reaches games/books/feeds only through
exported Go methods on those apps' structs (e.g. `Games.BuildSharedSteam`,
`Books.BuildSharedLibrary`, `Feeds.BuildSummary`) — never their internal packages
or schemas directly, since Go's `internal/` visibility rules block that anyway.

`games`/`books` no longer have any dashboard-shaped route or public-sharing code
of their own, only library/detail/settings pages. On the web side,
`app/dashboard/{games,reading}/` holds both the private (owner) and public
(token-shared) views.

## Alternatives considered

### Keeping dashboards in each owning app

Rejected: the share-token lifecycle was duplicated per app, and a cross-app
"Reading" dashboard spanning books *and* feeds had no natural home.

### Giving `dashboard` its own schema and copying data into it

Rejected: it would need invalidation on every upstream write. Reading through
exported methods keeps one source of truth per domain.

## Consequences

- Anything `dashboard` needs from another app must be **exported** on that app's
  struct. This is a deliberate, visible coupling point rather than a hidden
  schema read.
- Two invariants hold: `dashboard` owns no schema, and it never reaches into
  another app's `internal/` or schema.

## Revisit when

A third consumer needs the same cross-app read methods, which would argue for a
shared read model rather than app-struct methods.

## Implementation invariants

`dashboard.v1.PublicGamesDashboardService` and
`dashboard.v1.PublicReadingDashboardService` (`apps/dashboard/connect_public.go`)
are registered **without any auth middleware**. Every request carries an opaque
share token (`global.profile_shares`, keyed by `(user_id, app)` where `app` is
`'games'`/`'reading'`) that resolves to the owning user.

- **Public handlers must never read the user-context key**, since no middleware
  sets it.
- Each handler resolves the token, then delegates to an exported method on the
  live `*games.Games`/`*books.Books`/`*feeds.Feeds` reference `dashboard` was
  constructed with (`BuildSharedSteam`, `BuildSharedLibrary`, `BuildSharedFeeds`,
  …) rather than duplicating any business logic.

The owner manages both dashboards' tokens (plus their public display name)
through `dashboard.v1.DashboardService` (`cmd/api/connect_dashboard.go`), behind
normal `Access` — deliberately **not** gated by `dashboard`'s own `AppAccess`, so
a user's ability to share their games/reading dashboard doesn't depend on a
separate `dashboard` app-access grant. See that file's doc comment.

`dashboard`'s own `Routes()` therefore only ever registers the two public
services above.
