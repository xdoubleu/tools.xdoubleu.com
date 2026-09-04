# ADR-0018: A delisted game counts unless a listed game took its achievements

- Status: Accepted
- Issues: #1375, #1424
- Affects: `api/apps/games/internal/services/steam.go`, `api/apps/games/internal/services/progress.go`, `api/apps/games/internal/repositories/steam.go`, `api/apps/games/migrations/00005_in_completion_average.sql`

## Context

The games dashboard's "Current rate" tile and its distribution chart reproduce
the "Avg. Game Completion Rate" on the owner's Steam profile. Both average one
population: the games the user has unlocked at least one achievement in.

`is_delisted` is set by `buildGamesMap` for any stored game that
`IPlayerService/GetOwnedGames` no longer returns. That happens for two quite
different reasons, and the app treated them the same:

| app | why it left the owned list | on the profile |
|---|---|---|
| `380` Half-Life 2: Episode One | Valve folded it into Half-Life 2 (`220`) | not counted |
| `420` Half-Life 2: Episode Two | Valve folded it into Half-Life 2 (`220`) | not counted |
| `214850` GameMaker: Studio | retired, no successor app | **counted** |

The app's rule moved twice on inference before anyone checked the profile:

- #1375 excluded all delisted games (160 games at 40.06% → 157 at 39.62%).
- #1424 nearly reverted that and counted them all again, because 160 games
  average to 40.08% and the profile showed "40".

Neither reading was checked against Steam. What settled it was two numbers off
the profile itself: **157 games showing achievement progress**, and **22 perfect
games**. The app had 157 listed games with progress of which 21 were perfect, so
the profile counts one extra perfect game — GameMaker: Studio — and does not
count the two episodes.

The reason is visible in the data. Half-Life 2 (`220`) is still in the library
and now carries 69 achievements: the base game's 33 plus every `EP1_*` and
`EP2_*` name that `380` and `420` hold separately. Valve moved the episodes'
achievements into the base app. Counting the episode apps as well would put the
same achievements into the average twice. Nothing took GameMaker: Studio's
`GMS_*` achievements over, so they still stand on their own — and Steam still
counts them.

## Decision

A game takes part in the library-wide averages when Steam still lists it, **or**
when it is delisted and no listed game carries its achievements.

`markCompletionAverageMembership` (`services/steam.go`) decides this on every
sync by comparing achievement API names, and stores the verdict in
`steam_games.in_completion_average`. Containment must be complete and within one
listed game: achievement API names are only unique per app, so a couple of
generic ones (`ACH_01`) shared across unrelated games must not read as a
takeover.

`GetAveragedGames` returns that population, and both the distribution chart and
the progress graph use it, so the chart and the tile beside it can never
describe different libraries.

`is_delisted` keeps its own single meaning — *`GetOwnedGames` no longer returns
this app ID* — and still drives presentation on its own: the three backlog
lists filter it out, and `SteamResponse.delisted` reports those games with their
`in_completion_average` flag, since they appear in no list.

## Alternatives considered

### Excluding every delisted game (#1375)

Rejected: it drops GameMaker: Studio, which the profile counts — that is the
missing 22nd perfect game, and 157 games at 39.64% against 158 at 40.03%.

### Counting every delisted game (#1424's first attempt)

Rejected: it double-counts the Half-Life 2 episodes' achievements, which
Half-Life 2 already contributes.

### Hard-coding the app IDs

Rejected. Valve folds and retires apps continually; a list of app IDs is stale
the day it is written, and the achievement names already say which case a game
is in.

## Consequences

- The averages and the three backlog lists deliberately cover different
  populations, and neither is the other's total.
- A superseded game keeps its detail page and its stored rate; it just stops
  feeding the averages.
- `in_completion_average` defaults to `TRUE`, so every existing row keeps
  counting until the next sync recomputes it. For a delisted game that is the
  wrong answer for one sync cycle — an accepted cost over backfilling a
  judgement that needs achievement names to compute.
- The comparison is O(delisted × listed) set containments per sync, on data the
  sync already holds in memory.

## Revisit when

**Never on a whole-number reading of the profile alone.** The profile renders a
whole number, which hides up to a full point either way, and this number has now
moved twice on that evidence. What settles a question about it:

- *Which games are counted* — the profile's game count and its **Perfect Games**
  count, against `SteamResponse.delisted` and its `in_completion_average` flags.
  Perfect Games is the sharper of the two: it is stated outright rather than
  counted by hand.
- *Whether the app missed something* — find the achievement that moved the
  profile, work out its arithmetic weight, and look for exactly that step in the
  stored progress graph. Two Breathedge achievements on 2026-09-03 were worth
  `2 / 54 / 157 = 0.024pp`, and the graph moved 39.62 → 39.64: the same event,
  the same size. A profile that "jumps" on a step that small has crossed a
  rendering threshold, not gained a point.
