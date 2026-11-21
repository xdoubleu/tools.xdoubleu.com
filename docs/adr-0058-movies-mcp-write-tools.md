# ADR-0058: movies gets MCP write tools for the backlog

- Status: Accepted
- Affects: `api/apps/movies/mcp/mcp.go`, `api/apps/movies/mcp/write.go`, `api/apps/movies/internal/services/titlechange.go`

## Context

The movies tools were read-only, so an agent that found a title with
`movies_search_titles` could not add it or change its status without driving
the web UI behind a login. ADR-0023 requires a separate justification and ADR
for each app's mutating tools.

## Decision

- `movies_add_title`, `movies_update_title` and `movies_remove_title` wrap the
  existing `MovieService` calls, so entries are identical to ones added from the
  site (TMDB details, seasons and providers are stored the same way).
- Trust model as ADR-0023/0026: `RequireAppWrite(ctx, "movies")`; the user
  comes from context, and `shared` selects the family list as in the read tools.
- Adding is idempotent: `MovieService.Ensure` returns the entry already in the
  list, or adds it as `want`. The RPC's re-add overwrites the status, so the
  tool only changes an existing entry's status, rating or watch date when the
  argument is passed.
- A title is watched through the same state the UI uses: the entry's status
  plus its dated `watched_at` array (a series keeps its watches per season).
  `movies_get_stats` counts those watches. `watched_at` dates a movie's
  latest watch (adding one when there is none); for a series, `season` +
  `season_watched` ticks a season and `watched_at` dates that tick.
- Ratings are 1-5; `0` clears. Status is `want`, `watching`, `watched` or
  `dropped`, as `movies_list_backlog` filters on.

## Alternatives considered

- **Wrapping `AddTitle`/`SetStatus` as they are**: `AddTitle` rejects
  `watching`/`dropped` and overwrites an existing status.
- **Add-only, no remove**: a mistaken add would need the UI to undo.

## Consequences

- A `movies` write grant lets an agent change the caller's (or, with `shared`,
  the family's) backlog, including deleting entries and their watch history.
- A rejected combination in `movies_add_title` for a new title (e.g.
  `watched_at` on a series) fails after the title was added as `want`.
- `mcp_test.go` covers idempotency, validation, dated watches
  reaching `movies_get_stats`, seasons and the shared list.

## Revisit when

Agents need rewatch history (several dated watches) or watch-date edits beyond
the latest one.
