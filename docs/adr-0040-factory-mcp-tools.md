# ADR-0040: The board's MCP tools are for agent accounts and can't mark a task Ready

- Status: Accepted
- Issues: #2714, #2701
- Affects: `api/apps/factory/mcp.go`, `api/internal/mcptools/` (`RequireAgent`,
  `AddAgentReadTool`, `AddAgentWriteTool`, `AgentToolsOnly`)

## Context

Agents file and discuss tasks on the factory board, but a person alone decides
that work may start: only a human admin in a browser moves a task into Ready
([ADR-0039](adr-0039-agent-accounts.md), #2701 decision 5). The existing tool
registrars gate on per-app access, which admits an admin and refuses an agent,
the opposite of what the board needs.

## Decision

- Six tools on `/apps/mcp`: `factory_list_repos`, `factory_list_tasks`,
  `factory_get_task` (read) and `factory_create_task`, `factory_comment_task`,
  `factory_move_task` (write). `factory_list_jobs` waits for jobs to exist
  (#2706).
- Each tool is gated by `RequireAgent`: the `agent` role only. An admin or a user
  gets a permission error, and `AgentToolsOnly` also leaves the `factory_` tools
  out of every non-agent's tool list.
- The tools call the services as an agent actor (`agent:<user id>`), which is
  neither a browser nor an admin. A move into Ready is refused by
  `TaskService.Move`, the same guard as the web UI, not by the tool, so no tool
  can forget it.
- `factory_list_tasks` filters by status, repo, label (any case) and `query`,
  a case-insensitive search of titles and bodies run in SQL, so a routine finds
  the task it filed earlier by a stable key (a Sentry permalink) without
  reading every body.
- A new task always starts in Backlog. `factory_list_repos` returns id, owner,
  name and default branch only: the setup commands and instructions are for the
  sandbox, not the agent. List results leave out task bodies.

## Alternatives considered

- Gate on app access like other apps: an admin would pass and the agent fail.
- Check the status in the tool: a second path to forget; the service already owns
  the rule.

## Consequences

- An agent can create tasks, comment and move a task between every column except
  Ready, including out of Ready and into Done. Review of what it did stays with
  the person who merges.
- A person works the board through the web UI, not MCP.

## Revisit when

An agent needs to edit a task's body (the executor reads it frozen at Ready), or
when jobs exist and agents should see them.
