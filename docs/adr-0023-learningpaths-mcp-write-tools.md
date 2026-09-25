# ADR-0023: learningpaths' MCP tools are allowed to mutate

- Status: Accepted
- Issues: #1473 (part of epic #1471)
- Affects: `api/apps/learningpaths/mcp.go`, `api/cmd/api/mcp_apps.go`, `api/internal/mcptools/`

## Context

Every per-app MCP tool is read-only (`mcptools.AddReadTool` behind
`RequireAppAccess`). learningpaths exists so an agent can author curricula,
which read-only tools can't deliver.

## Decision

learningpaths adds three mutating tools (`learningpaths_create_path`,
`learningpaths_update_path`, `learningpaths_record_progress`) beside its three
read tools, with no change to the trust model:

- Each goes through `mcptools.RequireAppAccess(ctx, "learningpaths")`, the same
  gate as read tools.
- User scope comes from context: the tools wrap the Connect handlers, which call
  `getUser(ctx)`. There is no `user_id` argument and no cross-user write, admin
  or not.
- A local `addWriteTool` in `learningpaths/mcp.go` mirrors `AddReadTool`;
  `internal/mcptools` stays read-only.

## Alternatives considered

- **Read-only tools, mutate via UI** — defeats agent authoring.
- **Shared `AddWriteTool` now** — speculative with one caller.
- **Stricter write gate** — `RequireAppAccess` already bounds the same data;
  a second concept only adds sync burden.

## Consequences

- A `learningpaths` app-access grant is also write access to that user's own
  paths via MCP.
- `TestMCPTools_ReadAndWrite` (`mcp_internal_test.go`) proves writes go through
  the same service layer as the RPCs.

## Non-precedent statement

**This is not a precedent.** Any other per-app mutating MCP tool needs its own
real agent-authoring justification and its own ADR. The default stays
read-only.

## Revisit when

A second app earns a mutating tool — then consider a shared helper informed by
two real call sites.
