# ADR-0023: learningpaths' MCP tools are allowed to mutate

- Status: Accepted
- Issues: #1473 (part of epic #1471)
- Affects: `api/apps/learningpaths/mcp.go`, `api/cmd/api/mcp_apps.go`,
  `api/internal/mcptools/`

## Context

Every other app's MCP tool set (`recipes_*`, `books_*`, `feeds_*`,
`shoppinglist_*`, `games_*`, `trains_*`, `mealplans_*`) is read-only —
`api/internal/mcptools.AddReadTool` wraps a per-app read RPC behind
`RequireAppAccess` and returns JSON, with no write path. The two existing
mutations on the combined `/apps/mcp` server, `resolve_sentry_issue` and
`dismiss_security_alert`, are admin-only observability tools with no per-app
equivalent — `api/CLAUDE.md` states the invariant plainly: "no per-app tool is
ever mutating."

learningpaths (#1471/#1472) breaks that invariant on purpose.
Agent-authored learning curricula is the feature, not an add-on: a user asks a
Claude Code session (or any other MCP client) to draft or edit a learning
path, and the agent needs to actually create/update it and check items off —
not describe what the user should paste into the web UI. Read-only tools
alone cannot deliver that; #1473 exists specifically to add the three
mutating tools (`learningpaths_create_path`, `learningpaths_update_path`,
`learningpaths_record_progress`) alongside the three read tools
(`learningpaths_list_paths`, `learningpaths_get_path`,
`learningpaths_get_progress`).

## Decision

`learningpaths` gets mutating MCP tools. Nothing about the underlying trust
model changes to make this safe — the tools reuse exactly the same gate and
scoping every read tool already relies on:

- **Every mutating tool goes through `mcptools.RequireAppAccess(ctx,
  "learningpaths")`** — admin, or `learningpaths` in the caller's app-access
  list — the identical gate `AddReadTool` applies. Write tools get no looser
  a check than read tools.
- **User scoping is always derived from context, never from a tool
  argument.** Each mutating tool wraps the app's own Connect RPC handler
  (`CreateLearningPath`/`UpdateLearningPath`/`RecordItemProgress`), and those
  handlers call `getUser(ctx)` internally — there is no `user_id` parameter
  a caller could set to address someone else's path. This is the same
  reasoning `AddReadTool`'s doc comment already gives for why a read tool
  grants "exactly the access the caller already has over HTTP — no more":
  the write tools grant exactly the same, for mutation.
- **No admin-only bypass and no cross-user writes.** An admin caller can
  mutate because admins pass `RequireAppAccess` for every app, the same as
  they can read every app's data today — not because a separate
  admin-mutation path exists. There is no tool that lets one user create or
  edit another user's learning path, admin or not.
- **No new shared write-tool framework.** `learningpaths/mcp.go` registers
  its three mutating tools with a local `addWriteTool` helper that mirrors
  `mcptools.AddReadTool` (and `cmd/api/mcp_apps.go`'s `addObsTool`) almost
  exactly — same gate, same JSON result marshaling — kept local to this
  package rather than promoted into `internal/mcptools`, since this app is
  the only caller and a shared abstraction with one caller is speculative.
  `internal/mcptools` itself stays read-only.

## Alternatives considered

- **Keep learningpaths MCP tools read-only, mutate only through the web
  UI.** Defeats the point: the epic's whole premise is that an agent session
  authors the path, not a human clicking through a form. Rejected.
- **A generic `mcptools.AddWriteTool[In]` helper from day one.** Nothing else
  needs it yet, and guessing its shape (does it need a different result
  type? audit logging? rate limiting?) ahead of a second real caller is the
  kind of speculative abstraction this issue's own "Not this" section rules
  out. If a second app earns mutating MCP tools later, generalize then.
- **A separate, more restrictive gate for write tools (e.g. a new
  `learningpaths`-only scope, or requiring a confirmation round-trip).**
  `RequireAppAccess` already is the per-user boundary this repo trusts for
  read access to the same data; inventing a second, stricter concept for
  write access to the *same* data doesn't add real safety, only a second
  thing to keep in sync with `auth.AppAccess`. Not built.

## Consequences

- `api/CLAUDE.md`'s Apps MCP Server description now carries an explicit
  carve-out instead of an absolute "read-only" claim — anyone skimming that
  file sees the exception named at the point where the invariant is stated,
  not just here.
- A `learningpaths` app-access grant is now a grant of **write** access to
  that user's own learning paths via MCP, not just read access — worth
  remembering when reasoning about what any given app-access list actually
  authorizes. The scope is still just that one user's data.
- The three mutating tools are exercised by
  `api/apps/learningpaths/mcp_internal_test.go`
  (`TestMCPTools_ReadAndWrite`), which creates, updates, and records progress
  on a path purely through the MCP tool wrappers and confirms the read tools
  see the result — proving the write path lands in the same service layer
  as the Connect RPCs, not a shortcut around it.

## Non-precedent statement

**This is not a precedent.** No other app's MCP tools become mutating by
citing this ADR. Any future per-app mutating MCP tool needs its own explicit
justification — a real agent-authoring use case, not "it would be
convenient" — and its own equivalent of this document, because the default
for every app in this repo remains: MCP tools are read-only.

## Revisit when

A second app wants a mutating MCP tool. At that point, and not before,
reconsider whether `addWriteTool`'s duplicated-per-app shape (this file, plus
whatever the second app writes) is worth collapsing into a shared
`internal/mcptools` helper — with two real call sites informing what that
helper actually needs to support, rather than one imagined in advance.
