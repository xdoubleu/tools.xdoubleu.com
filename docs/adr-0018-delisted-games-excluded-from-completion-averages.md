# ADR-0018: Delisted games stay out of the completion averages

- Status: Accepted
- Issues: #1375, #1424
- Affects: `api/apps/games/internal/services/steam.go`, `api/apps/games/internal/services/progress.go`, `api/apps/games/internal/repositories/steam.go`

## Context

The games dashboard's "Current rate" tile and its distribution chart are meant
to reproduce the "Avg. Game Completion Rate" on the owner's Steam profile.
Both average one population: the games the user has unlocked at least one
achievement in.

`is_delisted` is set by `buildGamesMap` for any stored game that
`IPlayerService/GetOwnedGames` no longer returns — Valve folding one app into
another (`380`/`420`, the Half-Life 2 episodes) or retiring it outright
(`214850`, GameMaker: Studio). #1375 excluded those games from the averages,
bringing the headline rate from 40.06% over 160 games to 39.62% over 157.

Three days later the profile read 40% while the app read 39.64%, and the
disagreement looked like #1375 pointed the wrong way. It does not. **The owner
counted the games showing achievement progress on the profile: 157** — exactly
the non-delisted population. #1375's rule is right, and #1424 nearly reverted
it on circumstantial evidence.

The two readings that made it look wrong are explained by what Steam renders,
not by which games it counts. The profile figure is a whole number, so a
difference of a few tenths — and which side of a boundary each value sits on —
is invisible: 40.06 and 39.62 both read as plausible against a profile showing
"39", and 39.64 reads as "39" beside a profile showing "40".

## Decision

Library-wide Steam averages count **only games Steam still lists in the
library**: `GetActiveGames` feeds the distribution chart, and `activeGames` /
`activeAchievements` filter the progress graph and the per-game contribution
denominator on both the full-sync and single-game refresh paths.

`is_delisted` means exactly one thing — *`GetOwnedGames` no longer returns this
app ID*. Delisted games keep their stored rate and their own detail page, and
are excluded from the three backlog lists.

Because that leaves them invisible in every list and every number,
`SteamResponse` carries a `delisted` list so the population behind a completion
number can be checked directly rather than inferred.

## Alternatives considered

### Counting delisted games, on the grounds that the achievements stay on the profile

Rejected — this is what #1424 proposed. Steam does still return player
achievements for `380`/`420` through `GetPlayerAchievements`, and including
the three delisted games with achievements yields 40.08%, which matches the
"40" the profile showed on 2026-09-04. Both facts are true and neither is
evidence: the profile counts 157 games, so the match at 40.08 is a
coincidence of two numbers a third of a point apart being rendered as the same
whole number.

### Also hiding delisted games from their own detail page

Rejected. The achievements were earned; the game is only absent from the
current library.

## Consequences

- The headline rate and the distribution chart average 157 games while the
  three backlog lists cover 201 (they include games at 0%). Those populations
  are deliberately different and neither is the other's total.
- A delisted game's stored rate is frozen: Steam answers `403 Profile is not
  public` for a retired app's player stats, which is why `214850`, `502820`,
  `1549180` and `2767030` log a warning on every sync.

## Revisit when

**Never on a whole-number reading of the profile alone.** This number has now
been argued over twice from a rendered "39" or "40" that hides up to a full
point either way, and was nearly reverted the second time. Before changing the
rule, establish the population the same way the owner did — count the games
with achievement progress on the profile — and compare it against the
`delisted` list in `SteamResponse`.
