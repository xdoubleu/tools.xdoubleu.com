# ADR-0018: A delisted game counts unless a listed game took its achievements

- Status: Accepted
- Issues: #1375, #1424
- Affects: `api/apps/games/internal/services/steam.go`, `api/apps/games/internal/services/progress.go`, `api/apps/games/internal/repositories/steam.go`, `api/apps/games/migrations/00005_in_completion_average.sql`

## Context

The dashboard's "Current rate" tile and distribution chart reproduce Steam's
profile "Avg. Game Completion Rate", averaged over games with at least one
unlocked achievement. `is_delisted` marks games `GetOwnedGames` no longer
returns, for two different reasons:

| app | why it left | on the profile |
|---|---|---|
| `380`, `420` Half-Life 2 Episodes | folded into Half-Life 2 (`220`) | not counted |
| `214850` GameMaker: Studio | retired, no successor | **counted** |

The rule flipped twice on inference from the rounded percentage (#1375 excluded
all delisted; #1424 nearly counted all). The profile's own counts settled it —
**157 games with progress, 22 perfect** versus the app's 157 listed with 21
perfect: GameMaker counts, the episodes don't. Half-Life 2 now carries the
episodes' `EP1_*`/`EP2_*` achievements, so counting the episode apps would
double-count them.

## Decision

A game is in the averages if Steam still lists it, **or** it's delisted and no
single listed game contains its full achievement set.

- `markCompletionAverageMembership` (`services/steam.go`) computes this each
  sync by API-name containment and stores `steam_games.in_completion_average`.
  Partial overlap or a set spread over several games isn't a takeover. Generic
  names (`ACH_01`) may false-match; accepted over a name heuristic.
- `GetAveragedGames` returns that population for both the chart and the
  progress graph, so they always agree.
- `is_delisted` keeps its one meaning; backlog lists filter it out and
  `SteamResponse.delisted` reports those games with their flag.

## Alternatives considered

- **Exclude all delisted** — drops GameMaker, the missing 22nd perfect game.
- **Count all delisted** — double-counts the episodes.
- **Hard-code app IDs** — stale immediately; achievement names already tell.

## Consequences

- Averages and backlog lists cover different populations deliberately.
- `in_completion_average` defaults `TRUE`, so a delisted game is wrong for one
  sync cycle — accepted over backfilling.
- O(delisted × listed) containment checks per sync, in memory.

## Revisit when

**Never on the profile's whole-number percentage alone** — it hides up to a
point either way. Check the profile's game count and **Perfect Games** count
against `SteamResponse.delisted`. To trace a movement, compute the triggering
achievement's weight and find that step in the progress graph (e.g. two
achievements = `2 / 54 / 157 = 0.024pp`, graph 39.62 → 39.64).
