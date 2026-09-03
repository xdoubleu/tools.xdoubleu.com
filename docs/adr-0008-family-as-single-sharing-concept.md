# ADR-0008: One `family` concept replaces per-app sharing and standalone contacts

- Status: Accepted
- Issues: #1349, #1403
- Affects: `api/internal/family/`, `api/internal/repositories`, `apps/recipes`, `apps/mealplans`, `apps/shoppinglist`, `web/app/family`

## Context

Sharing was per-app and owner-centric, and a separate standalone `contacts`
concept existed alongside it. Two overlapping models for "who else can see this"
meant every new app had to pick one and reimplement it.

## Decision

`internal/family` is **the** single sharing concept, replacing per-app
owner-centric sharing *and* the former standalone `contacts` (removed in #1403
once it gated access to nothing).

### Data model

- `global.families` / `global.family_members` — one row per user, at most one
  family each. A user with no row is an **implicit family-of-one**, lazily
  materialized by `FamilyRepository.EnsureFamily` the first time it's asked for.
  Each row also carries the member's own `display_name`, shown to the rest of the
  family (migration `00041`).
- `global.family_invites` — **pending-only**. Accepting or declining always
  deletes the row rather than marking it resolved, since at most one family per
  user means at most one pending invite per invitee.

### Service

`family.v1.FamilyService`:
`GetFamily`/`InviteToFamily`/`AcceptFamilyInvite`/`DeclineFamilyInvite`/
`SetFamilyDisplayName`/`LeaveFamily` (frontend at `web/app/family`).

`InviteByEmail` requires the invitee to **already be a registered user** (looked
up via `auth.GetAllUsers`) and emails them off the request path — a send failure
is logged, never fails the request.

recipes/mealplans/shoppinglist key their data by `family_id`, via
`repositories.FamilyRepository` passed into each `app.go`.

## Alternatives considered

### Keeping `contacts` alongside families

Rejected in #1403: by then it gated access to nothing, so it was pure duplicate
surface area.

### Allowing a user to belong to multiple families

Rejected: at most one family per user is what makes "at most one pending invite"
and the implicit family-of-one both hold, keeping invite state a single
pending-only table.

### Un-merging data on leave

**Explicitly rejected (#1349's confirmed decision).** Leaving a family cannot
un-merge already-family-scoped data — the leaving user loses their membership row
and starts over as a fresh solo family.

## Consequences

- Data created while in a family stays with that family. This is irreversible by
  design and must be communicated in any leave-family UI.
- A new app that needs sharing takes `FamilyRepository` and keys by `family_id`;
  it must not invent its own sharing model.

## Revisit when

A genuine multi-group requirement appears (e.g. a household *and* a separate
shared-with-friends scope), which would break the one-family-per-user invariant
this design rests on.
