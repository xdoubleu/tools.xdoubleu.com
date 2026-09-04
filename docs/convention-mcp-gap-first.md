# Convention: fix the missing MCP tool before investigating the incident

- Enforced by: nothing but review
- Issues: #1027, #1195, #1214, #1357, #1374, #1377, #1424

## Rule

If the user describes a production issue and there is **no MCP tool that surfaces
it**, or an existing tool returns wrong/incomplete data, **fix that gap first**
— add or correct the tool — before investigating the issue itself.

Then add the case to the log below.

## Why

Otherwise the same blind spot just recurs next time. Every entry below cost real
investigation time that a working tool would have made unnecessary, and several
were only answerable with direct database access.

## Case log

### #1027 — bytes, not just requests

Supabase restricted the whole project for blowing its monthly egress quota, and
nothing could say which endpoint caused it: `get_usage_stats` counted
**requests** per endpoint but never bytes, so an endpoint returning 2 MB a call
looked identical to one returning 200 B.

`global.usage_daily` gained a `bytes` column and `get_usage_stats` now reports
it. The database is reached over a transaction-mode pooler and billed per byte
returned, so "which endpoint moves the most data" is the question that matters —
see `convention-database-queries.md` for the query rule this exists to enforce.

### #1195 — OAuth connection state was invisible

Reconnecting GitHub in the monitoring app kept landing back on a "Connect"
button, and no tool could say why: nothing reported OAuth connection state at
all, so the cause — `ListOAuthConnections` judging coverage by the provider's
echoed scope, which GitHub normalizes down to just `repo` — was invisible without
database access.

`get_oauth_connections` now reports each provider's connected state alongside its
requested, granted, and currently-required scopes: the three values that explain
a not-connected verdict.

### #1214 — no per-source notification toggle

`jobs.IssueNotifierJob`/`jobs.WeeklyDigestJob` email an admin about Sentry
issues, failing dependency PRs, and unhealthy feeds, but nothing could say
whether a given source was enabled or explain a missing email — there was no
per-source toggle at all, so "why didn't I get emailed" was unanswerable without
database access.

`global.notification_settings` now holds a per-source enabled flag the jobs check
before notifying, surfaced by `get_notification_settings` and toggled from the
monitoring page.

### #1357 — project board columns were unreadable

Asked to work through the project board's "Ready" issues, nothing could answer
"which issues are in that column". The separate GitHub MCP server's
`list_issue_fields`/`field_filters` only resolve custom fields on
**organization**-owned projects, and this repo's board
(`.claude/github-triage.config.json`'s `project.owner`, `xdoubleu`) is a personal
one — `list_issue_fields` errors "Could not resolve to an Organization" and
`field_filters` comes back empty for every issue.

`github.Client.ListProjectIssuesByStatus`
(`api/internal/github/project_issues.go`) queries GitHub's GraphQL API directly
with the admin's own connected OAuth token — not proxy-restricted, unlike raw
calls from an agent session — for a `ProjectV2` board's Status field, surfaced by
`get_project_issues_by_status`.

This required adding `read:project` to the GitHub OAuth connection's scopes
(`api/internal/github/oauth.go`). **An admin who connected GitHub before this
change needs to reconnect once.**

### #1374 / #1377 — an unreconcilable games number

The games dashboard's completion rate disagreed with Steam's own profile (40.06%
vs 39%), and answering "which games is each number averaging?" took a subagent
enumerating all 11 distribution buckets and hand-diffing appids against
`games_get_steam`'s three lists. Two blind spots caused that:

1. `games_get_steam_distribution`'s `bucket` argument was documented to agents as
   `0-9` while `services.DistributionLabels` has **11** entries, so bucket 10 —
   every 100%-completed game — was invisible to anything trusting the schema
   (#1377).
2. **Nothing surfaces delisted games at all.** `games_get_steam`'s lists all
   filter `is_delisted`, so the three games inflating the rate appeared in no
   tool's output.

Bucket 10 was fixed then. The delisted blind spot was not, and #1424 is the bill
for that.

### #1424 — the same delisted blind spot, three days later

The rate disagreed with the Steam profile again, in the other direction. With
no tool listing delisted games, the app IDs were once more only recoverable
from #1375's commit message, and checking them meant pulling
`games_get_steam_game` one at a time. On that evidence the rule from #1375 was
about to be reverted; the owner counting 157 games with achievement progress on
the profile is what stopped it — see
`adr-0018-delisted-games-excluded-from-completion-averages.md`.

`SteamResponse` now carries a `delisted` list, so `games_get_steam` reports the
games excluded from `current_rate`, `distribution` and all three lists. That
closes the gap #1374 left open, and makes the population behind a completion
number checkable instead of inferable.
