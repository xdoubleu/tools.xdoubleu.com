# Spec: the Apps MCP server

- Source of truth: `api/cmd/api/mcp_apps.go`, `api/cmd/api/apps.go`, `api/internal/mcptools/`, `apps/<name>/mcp.go`
- Issues: #1039, #1453

Auth for this server is ADR-0006. Setup instructions are in root `README.md`.

## Shape

Every app's own read RPCs, plus admin observability, are exposed to a local
Claude CLI over a largely read-only MCP server at `/apps/mcp`
(`cmd/api/mcp_apps.go`).

- Apps opt in via the `MCPToolProvider` interface
  (`RegisterMCPTools(srv *mcp.Server)`, `cmd/api/apps.go`).
- Each implementing app has its own `apps/<name>/mcp.go` wrapping only its
  **read** Connect handlers, exposed as `<app>_<rpc>` tools.
- `registerObservabilityMCPTools` adds 20 unprefixed admin-gated observability
  tools on top, sharing the exact same internal methods the Connect handlers use.
- Shared gating and marshaling live in `api/internal/mcptools`.

## Behavior

App tools are gated by the caller's own per-app access and return **only that
user's data**. Observability tools require admin.

`trains` is the one app whose tools are not per-user: its timetable is public
data shared by every caller, so `trains_search_stations`,
`trains_get_feed_info` and `trains_search_journeys` return the same rows to
anyone holding `trains` access. `trains_get_feed_info` reports `imported_at`
alongside the feed version, because a conditional GET makes an unchanged feed
a no-op and a stale import is otherwise indistinguishable from a current one
(issue #1453).

### The two deliberate mutations

18 of the 20 observability tools are read-only. Two are not:

- **`resolve_sentry_issue`** — marks a Sentry issue resolved via
  `sentryapi.Client.ResolveIssue`, letting an admin-authenticated agent close out
  an issue it just filed a fix for.
- **`dismiss_security_alert`** — dismisses/resolves an open Dependabot,
  code-scanning, or secret-scanning alert via
  `github.Client.DismissSecurityAlert`, letting an agent triaging
  `get_security_alerts` close one out directly instead of clicking through
  github.com's Security tab by hand. **The reason string GitHub's API requires
  differs per alert type** — see the tool's own description and
  `github.DismissSecurityAlert`'s doc comment for the exact accepted values.

## Invariants

- **No per-app tool is ever mutating.** `apps/<name>/mcp.go` wraps read handlers
  only.
- App tools never return another user's data.
- Observability tools are admin-gated without exception.

## Known gaps

See `convention-mcp-gap-first.md` for the running log of cases where a tool was
missing or wrong, and the rule that says to fix the gap before investigating the
incident behind it.
