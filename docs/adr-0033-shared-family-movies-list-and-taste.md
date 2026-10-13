# ADR-0033: A family keeps one shared movies list and taste, keyed like a user

- Status: Accepted
- Issues: #2474
- Affects: `api/apps/movies`, `api/apps/taste`, `api/internal/consumption`, `web/app/movies`, `web/app/taste`

## Context

Movies is per user, and taste (ADR-0032) never shared a profile. Two people who
watch together want one list and one taste profile for it, next to their own.

## Decision

- The shared list belongs to the whole family (ADR-0008), not a chosen pair.
  Its owner key is `consumption.SharedSubject(familyID)` (`family:<uuid>`),
  stored where a user ID goes, so the solo and shared lists reuse every query,
  status, rating and watch date. A title can sit on both lists independently.
- Movies and taste RPCs take `shared`; the handler maps it to the owner key
  (`FamilyRepository.EnsureFamily`). Any family member reads and edits the list.
- The shared taste profile and picks are generated under the same key from
  movies and series only. The profile job also covers each family with a
  shared title. Shared watches never feed a solo profile, and shared picks send
  no email.
- Streaming-service flags and bonuses use per-user settings, so the shared
  list has none.

## Alternatives considered

- **A pair of users** — needs a pairing concept beside families.
- **A `family_id` column** — touches every query, for the same result.
- **Sharing the solo list** — the solo list stays private.

## Consequences

- Data stays in the family when someone leaves (ADR-0008); the leaver starts a
  fresh solo family and loses access to the old shared list.
- Shared picks accepted from taste land in the shared list.
- A user ID starting `family:` would collide with a shared key; user IDs never do.

## Revisit when

Families need several shared lists, or the shared taste should include other
domains.
